#!/bin/env/python3

from __future__ import print_function
import json
import sys
import time
import argparse
import logging
import os
import math
import socket

from requests import get
from crontab import CronTab
from datetime import datetime, timedelta
from prometheus_client import Gauge, Counter, Summary, start_http_server
from logging import getLogger, StreamHandler, DEBUG
from multiprocessing import Pool


logger = getLogger(__name__)
handler = StreamHandler()
handler.setLevel(DEBUG)
logger.setLevel(DEBUG)
logger.addHandler(handler)
logger.propagate = False


def eprint(*args, **kwargs):
    print(*args, file=sys.stderr, **kwargs)


class JobConfig(object):
    """
    処理設定
    """

    def __init__(self, crontab):
        """
        :type crontab: crontab.CronTab
        :param crontab: 実行時間設定
        """

        self._crontab = crontab

    def schedule(self):
        """
        次回実行日時を取得する。
        :rtype: datetime.datetime
        :return: 次回実行日時を
        """

        crontab = self._crontab
        return datetime.now() + timedelta(
            seconds=math.ceil(
                crontab.next(default_utc=False)
            )
        )

    def next(self):
        """
        次回実行時刻まで待機する時間を取得する。
        :rtype: long
        :retuen: 待機時間(秒)
        """

        crontab = self._crontab
        return math.ceil(crontab.next(default_utc=False))


def job_controller(crontab):
    """
    処理コントローラ
    :type crontab: str
    :param crontab: 実行設定
    """
    def receive_func(job):
        import functools
        @functools.wraps(job)

        def wrapper():
            # Start up the server to expose the metrics.
            start_http_server(args.port, args.listen)
            # Generate some requests.
            jobConfig = JobConfig(CronTab(crontab))
            logging.info("->- Process Start")
            while True:
                try:
                    # 次実行日時を表示
                    logging.info("-?- next running\tschedule:%s" %
                    jobConfig.schedule().strftime("%Y-%m-%d %H:%M:%S"))
                    # 次実行時刻まで待機
                    time.sleep(jobConfig.next())

                    logging.info("-!> Job Start")

                    # 処理を実行する。
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

parser.add_argument(
    "-p",
    "--port",
    type=int,
    default=int(os.environ.get('EXPORTER_PORT', '9353')),
    help="listen port number. (default: 9353)",
)

parser.add_argument(
    "-i",
    "--interval",
    type=str,
    default=os.environ.get('EXPORTER_INTERVAL', '*/20 * * * *'),
    help="interval default second (default: */20 * * * *)",
)

parser.add_argument(
    "-v",
    "--debug",
    default=os.environ.get('EXPORTER_DEBUG', 'false'),
    help="log level. (default: False)",
    action="store_true"
)

args = parser.parse_args()


if args.debug == 'false':
    logging.basicConfig(
        format='%(asctime)s %(levelname)s: %(message)s',
        datefmt='%Y-%m-%dT%H:%M:%S%z',
        level=logging.INFO
    )
    logging.info('is when this event was logged.')
else:
    logging.basicConfig(
        format='%(asctime)s %(levelname)s: %(message)s',
        datefmt='%Y-%m-%dT%H:%M:%S%z',
        level=logging.DEBUG
    )
    logging.debug('is when this event was logged.')


isp_info = Gauge(
    'isp_info',
    'Information about my ISP which is in use.',
    ['hostname', 'ip_address']
)
isp_info_running = Gauge('isp_info_running', 'ISP Info is running when 1')
isp_info_last_run = Gauge('isp_info_last_run', 'ISP Info Last Checked/Updated')
isp_info_update_time = Summary('isp_info_update_time', 'Description of summary')


@job_controller(args.interval)
@isp_info_update_time.time()
def job1():
    """
    処理1
    """
    logging.info('Updating ISP_INFO.')
    ip ='127.0.0.1'
    hostname = 'unknown'

    with isp_info_running.track_inprogress():
        try:
            logging.info('Attempting to get external IP...')
            hostname = 'unknown'
            ip = get('https://api.ipify.org').text
            logging.info(f"Got IP of {ip}")
        except Exception as ex:
            logging.error(f"Error detecting IP Address: {ex}")

        try:
            logging.info('Attempting to resolve external IP...')
            hostnames = socket.gethostbyaddr(ip)
            logging.debug(f"hostnames: {hostnames}")
            hostname = hostnames[0]
        except Exception as ex:
            logging.error(f"Error Getting IP Hostname: {ex}")

        logging.info('Setting values.')
        try:
            isp_info_last_run.set_to_current_time()
            isp_info.labels(hostname, ip).set_to_current_time()
        except Exception as ex:
            logging.error(f"Unable to set values: {ex}")


def main():
    """
    """

    # ログ設定
    logging.basicConfig(
        level=logging.DEBUG,
        format="time:%(asctime)s.%(msecs)03d\tprocess:%(process)d"
        + "\tmessage:%(message)s",
        datefmt="%Y-%m-%d %H:%M:%S"
    )

    # 処理リスト作成
    jobs = [job1]

    # 処理を並列に実行
    p = Pool(len(jobs))
    try:
        for job in jobs:
            p.apply_async(job)
        p.close()
        p.join()
    except KeyboardInterrupt:
        pass


if __name__ == "__main__":
    main()
