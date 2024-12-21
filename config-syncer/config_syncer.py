import kopf
import kubernetes
import logging

@kopf.on.startup()
def configure(settings: kopf.OperatorSettings, **_):
    settings.posting.level = logging.WARNING
    settings.watching.connect_timeout = 1 * 60
    settings.watching.server_timeout = 10 * 60
    settings.persistence.finalizer = 'config-sync.collom.co.uk/config-sync-finalizer'
    settings.networking.error_backoffs = [10, 20, 30]


@kopf.index('secrets')
def tuple_keys(namespace, name, **_):
    return {(namespace, name): 'hello'}
