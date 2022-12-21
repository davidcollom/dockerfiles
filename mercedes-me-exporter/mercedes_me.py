#!/usr/bin/env python3

# from __future__ import print_function
import json
import sys
import time
import argparse
import logging
import os
import math
import base64
import requests

from kubernetes import config, client

from retry import retry
from crontab import CronTab
from datetime import datetime, timedelta
from prometheus_client import Gauge, Counter, start_http_server
from logging import getLogger, StreamHandler, DEBUG
from multiprocessing import Pool
from multiprocessing.pool import ThreadPool

URL_RES_PREFIX = "https://api.mercedes-benz.com/vehicledata/v2"

logger = getLogger(__name__)
handler = StreamHandler()
handler.setLevel(DEBUG)
logger.setLevel(DEBUG)
logger.addHandler(handler)
logger.propagate = False


def eprint(*args, **kwargs):
    print(*args, file=sys.stderr, **kwargs)


class JobConfig(object):

    def __init__(self, crontab):
        """
        :type crontab: crontab.CronTab
        :param crontab: Execution time setting
        """

        self._crontab = crontab

    def schedule(self):
        """
        Get the next execution date and time.
        :rtype: datetime.datetime
        :return: Next execution date and time
        """

        crontab = self._crontab
        return datetime.now() + timedelta(
            seconds=math.ceil(
                crontab.next(default_utc=False)
            )
        )

    def next(self):
        """
        Get the time to wait until the next execution time.
        :rtype: long
        :retuen: Standby time (seconds)
        """

        crontab = self._crontab
        return math.ceil(crontab.next(default_utc=False))


def job_controller(crontab):
    def receive_func(job):
        import functools
        @functools.wraps(job)

        def wrapper():
            # Generate some requests.
            jobConfig = JobConfig(CronTab(crontab))
            logging.info("[%s]->- Initial Job Run..", job.__name__)
            job()
            logging.info("[%s]-<- Initial Job Done.", job.__name__)

            # if job.__name__ == "fetch_metrics":
            #   logging.info("[%s]->- Starting web Server %s:%s", job.__name__, args.listen, args.port)
            #   # Start up the server to expose the metrics.
            #   start_http_server(args.port, args.listen)

            logging.info("[%s]->- Process Start", job.__name__)
            while True:
                try:
                    # Display next execution date and time
                    logging.info("[%s]-?- next running\tschedule:%s",
                      job.__name__, jobConfig.schedule().strftime("%Y-%m-%d %H:%M:%S")
                    )
                    # Wait until the next execution time
                    time.sleep(jobConfig.next())

                    logging.info("[%s]-!> Job Start", job.__name__)

                    # Execute the process.
                    job()

                    logging.info("[%s]-!< Job Done", job.__name__)

                except KeyboardInterrupt:
                    break

        logging.info("[%s]-<- Process Done.", job.__name__)
        return wrapper
    return receive_func


# Argument parameter


parser = argparse.ArgumentParser()

parser.add_argument(
    "-l",
    "--listen",
    type=str,
    default=os.environ.get('EXPORTER_LISTEN', '0.0.0.0'),
    help="listen port number. (default: 0.0.0.0)",
)

parser.add_argument("-p","--port", type=int,
  default=int(os.environ.get('EXPORTER_PORT', '9353')),
  help="listen port number. (default: 9353)",
)
parser.add_argument("-i", "--interval", type=str,
  default=os.environ.get('EXPORTER_INTERVAL', '*/20 * * * *'),
  help="interval default second (default: */20 * * * *)",
)
parser.add_argument("-v", "--vin", type=str,
  default=os.environ.get('EXPORTER_VIN', ''),
  help="Vehicle Identification Number",
)
parser.add_argument("-t", "--secret",type=str,
  default=os.environ.get('EXPORTER_SECRET', 'mercedesme'),
  help="Authentication Secret",
)
parser.add_argument("-n", "--namespace",type=str,
  default=os.environ.get('NAMESPACE', 'monitoring'),
  help="Namespace for secret",
)

parser.add_argument("-d", "--debug", action="store_true",
  default=os.environ.get('EXPORTER_DEBUG', 'false'),
  help="log level. (default: False)"
)
args = parser.parse_args()


loglevel = logging.INFO
if args.debug == 'false':
  loglevel = logging.DEBUG

logging.basicConfig(
  format='%(asctime)s %(levelname)s: %(message)s',
  datefmt='%Y-%m-%dT%H:%M:%S%z',
  level=loglevel
)
metrics={}
metrics['odo_value'] = Gauge(
  'mercedes_me_odo_value',
  "Odometer Value (km)"
)
metrics['odo_timestamp'] = Gauge(
  'mercedes_me_odo_timestamp',
  "Odometer timestamp last updated"
)

metrics['fueltank_percentage'] = Gauge(
  'mercedes_me_fuel_percentage',
  "Fuel tank percentage full"
)
metrics['fueltank_timestamp'] = Gauge(
  'mercedes_me_fuel_timestamp',
  "Fuel tank percentage last updated"
)

metrics['range_value']  = Gauge(
  'mercedes_me_range_remaining',
  "Fuel tank range (km)"
)
metrics['range_timestamp'] = Gauge(
  'mercedes_me_range_remaining_timestamp',
  "Fuel tank range (km)"
)

metrics['tokens_refreshed'] = Counter(
  'mercedes_me_tokens_refreshed',
  "Refreshed Authentication Token"
)

metrics['exporter_up'] = Gauge(
  'mercedes_me_exporter_up',
  'mercedes_me_exporter is up(1) or down(0)'
)

access_token=None

def make_request(uri):
  # Set Header
  headers = {
    "accept": "application/json;charset=utf-8",
    "authorization": f"Bearer {access_token}",
  }
  # Send Request
  res = requests.get(uri, headers=headers)
  try:
    data = res.json()
  except ValueError:
    data = {"reason": "No Data", "code": res.status_code}
  # Check for any Error
  if not res.ok:
    if "reason" in data:
      reason = data["reason"]
    else:
      if res.status_code == 204:
        reason = "No Data Provided"
      elif res.status_code == 400:
        reason = "Bad Request"
      elif res.status_code == 401:
        reason = "Invalid or missing authorization in header"
      elif res.status_code == 402:
        reason = "Payment required"
      elif res.status_code == 403:
        reason = "Forbidden"
      elif res.status_code == 404:
        reason = "Page not found"
      elif res.status_code == 429:
        reason = ("The service received too many requests in a given amount of time")
      elif res.status_code == 500:
        reason = "An error occurred on the server side"
      elif res.status_code == 503:
        reason = "The server is unable to service the request due to a temporary unavailability condition"
      else:
        reason = "Generic Error"
    data["reason"] = reason
    data["code"] = res.status_code
  return data

def res_uri(resource,vin):
  return "/".join([URL_RES_PREFIX,'vehicles', vin, 'resources',resource])

class NoAccessTokenYet(Exception):
  pass

try:
  config.load_kube_config()
except:
  logging.info('Loading in cluster config')
  config.load_incluster_config()
v1 = client.CoreV1Api()

@job_controller(args.interval)
@retry(NoAccessTokenYet, delay=1, backoff=2, logger=logging)
def fetch_metrics():
    """
    Fetch Metrics
    """
    global metrics
    if access_token == None:
      logging.error(f"No Access token yet...[{access_token}]")
      raise NoAccessTokenYet

    results={}
    try:
      logging.info('Fetching Metrics.')

      try:
        # Odo Metrics:
        odo_uri = res_uri('odo', args.vin)
        odo_data = make_request(odo_uri)
        logging.info(f"ODO Data: {odo_data}")
        if odo_data.get('odo',{}).get('value') != None:
          metrics['odo_value'].set(odo_data['odo']['value'])
          metrics['odo_timestamp'].set(odo_data['odo']['timestamp'])
          logging.info("Set ODO Value to %s", odo_data['odo']['value'])

        # tanklevelpercent Metrics:
        tank_uri = res_uri('tanklevelpercent', args.vin)
        tank_data = make_request(tank_uri)
        logging.info(f"Tank Data: {tank_data}")
        if tank_data.get('tanklevelpercent',{}).get('value') != None:
          metrics['fueltank_percentage'].set(tank_data['tanklevelpercent']['value'])
          metrics['fueltank_timestamp'].set(tank_data['tanklevelpercent']['timestamp'])
          logging.info("Set Tank Value to %s", tank_data['tanklevelpercent']['value'])

        # rangeliquid Metrics:
        range_uri = res_uri('rangeliquid', args.vin)
        range_data = make_request(range_uri)
        logging.info(f"Range Data: {range_data}")
        if range_data.get('rangeliquid',{}).get('value') != None:
          metrics['range_value'].set(range_data['rangeliquid']['value'])
          metrics['range_timestamp'].set(range_data['rangeliquid']['timestamp'])
          logging.info("Set Range Value to %s", range_data['rangeliquid']['value'])

          metrics['exporter_up'].set(1)

      except Exception as ex:
        logging.error(ex)
        metrics['exporter_up'].set(0)

    except TypeError:
      logging.warning("Couldn't get results from speedtest-cli!")
      metrics['exporter_up'].set(0)

@job_controller('0 * * * *')
def refresh_token():
  global access_token
  global metrics
  logging.info(f"Fetching K8S Secret '{args.secret}'..")
  sec_data = v1.read_namespaced_secret(args.secret, args.namespace).data
  refresh_token=base64.b64decode(sec_data['TOKEN']).decode('UTF8')
  client_id=base64.b64decode(sec_data['CLIENT_ID']).decode('UTF8')
  secret_id=base64.b64decode(sec_data['CLIENT_SECRET']).decode('UTF8')
  try:
    logging.info("Refreshing AuthToken...")
    secheader = base64.b64encode( f"{client_id}:{secret_id}".encode('UTF8') ).decode('UTF8')
    headers={
      "Authorization":  f"Basic {secheader.replace(' ','')}",
      "content-type": "application/x-www-form-urlencoded",
    }
    data = {
      "grant_type": "refresh_token",
      "refresh_token": refresh_token,
    }
    logging.info(f"AuthToken Refresh... {headers} - {data}")
    res = requests.post(
      "https://id.mercedes-benz.com/as/token.oauth2",
      data=data,
      headers=headers
    )
    logging.info(f"Got the following response back: {res}")
    if res.ok:
      res_json = res.json()
      access_token = res_json.get('access_token', None)
      logging.info(f"Logging token set {type(access_token)}:{len(access_token)}")
      try:
        logging.info(f'Patching secret with new token...')
        resp = v1.patch_namespaced_secret(args.secret, args.namespace, body={
          'data': {
            'TOKEN': base64.b64encode(res_json.get('refresh_token', None).encode('UTF8')).decode('UTF8')
          }
        })
        logging.info("Token Successfully refreshed!")
        metrics['tokens_refreshed'].inc()
      except Exception as e:
        print("Exception when calling CoreV1Api->patch_namespaced_secret: %s\n" % e)
        return
    else:
      logging.error(f"Unable to refresh Auth token: {res}")
  except Exception as ex:
    logging.error(f"Unable to refresh Auth Token: {ex}")



def main():
    logging.basicConfig(
        level=logging.INFO,
        format="time:%(asctime)s.%(msecs)03d\tprocess:%(process)d"
        + "\tmessage:%(message)s",
        datefmt="%Y-%m-%d %H:%M:%S"
    )

    logging.info("->- Starting web Server %s:%s", args.listen, args.port)
    # Start up the server to expose the metrics.
    start_http_server(args.port, args.listen)

    # jobs = [fetch_metrics]
    # jobs = [refresh_token]
    jobs = [refresh_token, fetch_metrics]

    p = ThreadPool(len(jobs))
    try:
        for job in jobs:
            p.apply_async(job)
        p.close()
        p.join()
    except KeyboardInterrupt:
        pass


if __name__ == "__main__":
    main()
