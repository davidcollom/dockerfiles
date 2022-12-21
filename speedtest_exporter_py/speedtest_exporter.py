#!/bin/env/python3

from __future__ import print_function
import json
import sys
import time
import argparse
import logging
import os
import math
import speedtest

from crontab import CronTab
from datetime import datetime, timedelta
from prometheus_client import Gauge, Counter, start_http_server
from logging import getLogger, StreamHandler, DEBUG
from multiprocessing import Pool
from multiprocessing.pool import ThreadPool


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
            logging.info("->- Initial Job Run..")
            job()
            logging.info("-<- Initial Job Done.")

            logging.info("->- Starting web Server %s:%s", args.listen, args.port)
            # Start up the server to expose the metrics.
            start_http_server(args.port, args.listen)

            logging.info("->- Process Start")
            while True:
                try:
                    # Display next execution date and time
                    logging.info("-?- next running\tschedule:%s" %
                    jobConfig.schedule().strftime("%Y-%m-%d %H:%M:%S"))
                    # Wait until the next execution time
                    time.sleep(jobConfig.next())

                    logging.info("-!> Job Start")

                    # Execute the process.
                    job()

                    logging.info("-!< Job Done")

                except KeyboardInterrupt:
                    break

        logging.info("-<- Process Done.")
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

parser.add_argument("-p","--port",
    type=int, default=int(os.environ.get('EXPORTER_PORT', '9353')),
    help="listen port number. (default: 9353)",
)
parser.add_argument("-i", "--interval", type=str,
    default=os.environ.get('EXPORTER_INTERVAL', '*/20 * * * *'),
    help="interval default second (default: */20 * * * *)",
)
parser.add_argument("-d", "--debug", action="store_true",
    default=os.environ.get('EXPORTER_DEBUG', 'false'),
    help="log level. (default: False)"
)

parser.add_argument(
    "-s",
    "--server",
    type=str,
    default=os.environ.get('EXPORTER_SERVERID', ''),
    help="speedtest server id"
)

args = parser.parse_args()


loglevel = logging.INFO
if args.debug == True:
    loglevel = logging.DEBUG

logging.basicConfig(
    level=logging.DEBUG,
    format="time:%(asctime)s.%(msecs)03d\tprocess:%(process)d"
    + "\tmessage:%(message)s",
    datefmt="%Y-%m-%d %H:%M:%S"
)


speedtest_download_bits = Gauge(
    'speedtest_download_bits',
    'download bandwidth in (bit/s)'
)

speedtest_upload_bits = Gauge(
    'speedtest_upload_bits',
    'upload bandwidth in (bit/s)'
)

speedtest_download_bytes = Gauge(
    'speedtest_download_bytes',
    'download usage capacity (bytes)'
)

speedtest_upload_bytes = Gauge(
    'speedtest_upload_bytes',
    'upload usage capacity (bytes)'
)

speedtest_ping = Gauge(
    'speedtest_ping',
    'icmp latency (ms)'
)

speedtest_up = Gauge(
    'speedtest_up',
    'speedtest_exporter is up(1) or down(0)'
)

@job_controller(args.interval)
def fetch_metrics():
    """
    処理1
    """
    results={}

    global speedtest_download_bits
    global speedtest_upload_bits
    global speedtest_download_bytes
    global speedtest_upload_bytes
    global speedtest_ping
    global speedtest_up
    try:
        logging.info('Running speedtest-cli.')

        s = speedtest.Speedtest(secure=True)

        threads = None

        if args.server != '':
            logging.info('set server id: %s', args.server)
            s.get_servers([args.server])
        else:
            logging.info('finding best server...')
            s.get_best_server()

        logging.info('Running speedtest...')
        try:
            s.download(threads=threads)
            s.upload(threads=threads, pre_allocate=False)
            results = s.results.dict()
            logging.debug("Response: %s", results)

            speedtest_up.set(1)

        except Exception as ex:
            logging.warning("ERROR: Failed to parse JSON, all values will be 0!")
            logging.debug(ex)
            speedtest_up.set(0)

    except Exception as exp:
        logging.debug(exp)
        speedtest_up.set(0)

    except TypeError:
        logging.warning("Couldn't get results from speedtest-cli!")
        speedtest_up.set(0)

    # We set here, so that values can be set to zero on failure
    logging.info('Setting gauge values...')
    speedtest_download_bits.set( results.get('download',0) )
    speedtest_upload_bits.set( results.get('upload',0) )
    speedtest_download_bytes.set( results.get('bytes_received',0) )
    speedtest_upload_bytes.set( results.get('bytes_sent',0) )
    speedtest_ping.set( results.get('ping',0) )
    logging.info("Values Set!")


def main():
    """
    """

    # ログ設定



    jobs = [fetch_metrics]

    # 処理を並列に実行
    p = ThreadPool(5)
    try:
        for job in jobs:
            p.apply_async(job)
        p.close()
        p.join()
    except KeyboardInterrupt:
        pass


if __name__ == "__main__":
    main()
