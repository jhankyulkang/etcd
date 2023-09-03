import os

from fabric import task, Connection
from invoke import UnexpectedExit, run

#PASSWORD = os.environ['KHOURY_PASSWORD']
#SSH_KEY_PATH = '~/.ssh/id_rsa'
SSH_KEY_PATH = '/home/ubuntu/.ssh/id_rsa'

REMOTE_ETCD_DIR = '~/etcd'
REMOTE_SERVER_DIR = REMOTE_ETCD_DIR + '/server'

LOCAL_ETCD_DIR = '~/go/src/etcd'
LOCAL_SERVER_DIR = LOCAL_ETCD_DIR + '/server'

HTTP_SCHEME = 'http://'
LOCAL_HOST = '127.0.0.1'


class EtcdConfig:
    ip: str
    port: int
    name: str

    TickMs: int = 50
    ElectionMs: int = 1000

    listenClientUrls: str
    advertiseClientUrls: str

    initialAdvertisePeerUrls: str
    listenPeerUrls: str
    initialCluster: str
    initialClusterState: str = "new"

    preVote: str = "false"

    logLevel: str

    def join(self):
        self.initialClusterState = "existing"

    def __str__(self):
        return '--data-dir=' + 'data.etcd.' + self.name \
            + ' --name=' + self.name \
            + ' --heartbeat-interval=' + str(self.TickMs) \
            + ' --election-timeout=' + str(self.ElectionMs) \
            + ' --listen-client-urls=' + self.listenClientUrls \
            + ' --advertise-client-urls=' + self.advertiseClientUrls \
            + ' --initial-advertise-peer-urls=' + self.initialAdvertisePeerUrls \
            + ' --listen-peer-urls=' + self.listenPeerUrls \
            + ' --initial-cluster=' + self.initialCluster \
            + ' --initial-cluster-state=' + self.initialClusterState \
            + ' --pre-vote=' + self.preVote \
            + ' --log-level=' + self.logLevel


def parse_configs(cluster_url: str, merge: bool = False, logging: str = 'debug') -> list:
    if len(cluster_url) == 0:
        print("empty cluster url")
        exit(1)

    if logging not in ['debug', 'info', 'warn', 'panic', 'fatal']:
        print("invalid logging level")
        exit(1)

    configs = []
    for node in cluster_url.split(","):
        tokens = node.split("=")
        name = tokens[0]
        url = tokens[1]

        tokens = url.replace(HTTP_SCHEME, "").split(':')
        ip = tokens[0]
        port = int(tokens[1])

        config = EtcdConfig()
        config.ip = ip
        config.port = port
        config.name = name

        if merge:
            config.TickMs = 10
            config.ElectionMs = 100

        # client comm
        config.listenClientUrls = HTTP_SCHEME + ip + ':' + str(port - 1)
        config.advertiseClientUrls = HTTP_SCHEME + ip + ':' + str(port - 1)

        # peer comm
        config.initialAdvertisePeerUrls = url
        config.listenPeerUrls = url
        config.initialCluster = cluster_url

        config.logLevel = logging

        configs.append(config)

    return configs


@task
def start(ctx, cluster_url, merge='false', logging='debug'):
    """
    cluster_url format: 1=http://127.0.0.1:1380,2=http://127.0.0.1:2380,3=http://127.0.0.1:3380,4=http://127.0.0.1:4380,5=http://127.0.0.1:5380,6=http://127.0.0.1:6380
    """
    merge = merge.lower() == 'true'
    configs = parse_configs(cluster_url, merge, logging)
    for cfg in configs:
        start_config(cfg)

@task
def join2(ctx, cluster_url, names, logging='debug'):
    """
    cluster_url format: 1=http://127.0.0.1:1380,2=http://127.0.0.1:2380,3=http://127.0.0.1:3380,4=http://127.0.0.1:4380,5=http://127.0.0.1:5380,6=http://127.0.0.1:6380
    """
    configs = parse_configs(cluster_url, False, logging)
    names = set(names.split(','))
    for cfg in configs:
        if cfg.name in names:
            cfg.join()
            #print(cfg)
            start_config(cfg)


def start_config(cfg: EtcdConfig):
    ip = cfg.ip
    run_cmd(ip, 'cd {} && '.format(LOCAL_SERVER_DIR if ip == LOCAL_HOST else REMOTE_SERVER_DIR) +
            'nohup {} > etcd.{}.out 2>&1 &'.format('./server ' + str(cfg), cfg.name))
    print("started on " + ip)


@task
def stop(ctx, cluster_url):
    """
    cluster_url format: 1=http://127.0.0.1:1380,2=http://127.0.0.1:2380,3=http://127.0.0.1:3380,4=http://127.0.0.1:4380,5=http://127.0.0.1:5380,6=http://127.0.0.1:6380
    """
    configs = parse_configs(cluster_url)
    for cfg in configs:
        ip = cfg.ip
       # run_cmd(cfg.ip, 'kill -9 $({}lsof -t -i:{})'.format('' if ip == LOCAL_HOST else '/usr/sbin/', cfg.port))
        run_cmd(cfg.ip, 'killall server')
        print("stopped on " + ip)


@task
def clean(ctx, cluster_url):
    """
    cluster_url format: 1=http://127.0.0.1:1380,2=http://127.0.0.1:2380,3=http://127.0.0.1:3380,4=http://127.0.0.1:4380,5=http://127.0.0.1:5380,6=http://127.0.0.1:6380
    """
    stop(ctx, cluster_url)
    configs = parse_configs(cluster_url)
    for cfg in configs:
        ip = cfg.ip
        run_cmd(ip, 'cd {} && '.format(LOCAL_SERVER_DIR if ip == LOCAL_HOST else REMOTE_SERVER_DIR) +
                'rm -rf data.etcd.* && rm etcd.*.out')
        print("cleaned on " + ip)


@task
def logs(ctx, ip):
    """
    cluster_url format: 1=http://127.0.0.1:1380,2=http://127.0.0.1:2380,3=http://127.0.0.1:3380,4=http://127.0.0.1:4380,5=http://127.0.0.1:5380,6=http://127.0.0.1:6380
    """
    os.system('scp {}:{}/etcd.*.out .'.format(ip, REMOTE_SERVER_DIR))


def run_cmd(ip, cmd):
    if ip == LOCAL_HOST:
        run(cmd, asynchronous=True)
    else:
        with Connection(host=ip, user='jk', connect_kwargs={'key_filename': SSH_KEY_PATH}) as conn:
            try:
                print(cmd)
                conn.run(cmd, asynchronous=True)
            except UnexpectedExit:
                print("run cmd failed: " + cmd)
