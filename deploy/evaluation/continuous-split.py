import os
import subprocess
import time

import yaml

# Read yaml config
with open('continuous-split.yaml', 'r') as f:
    yamlf = yaml.safe_load(f)
    etcdctl_path = yamlf['etcdctlPath']
    server_path = yamlf['serverPath']
    endpoints = yamlf['endpoints']
    etcd_config = yamlf['etcdConfigs']
if len(endpoints) != 6:
    print('require 6 nodes')
    exit(1)


def execute_etcdctl(eps, arg):
    cmd = etcdctl_path \
          + ' --endpoints=' + ','.join([ep.removeprefix('http://') for ep in eps]) \
          + ' ' + arg
    data = subprocess.run(cmd, capture_output=True, shell=True)
    if data.returncode != 0:
        print('execute etcdctl error: ' + str(data.stderr, 'utf-8'))
        exit(1)
    return str(data.stdout, 'utf-8')


def kill(idx):
    ret = os.system('kill $(lsof -ti:{})'.format(endpoints[idx].split(':')[1]))
    if ret != 0:
        print('kill server {} failed')
        exit(1)


def start(idx):
    subprocess.Popen(['env', 'ETCD_CONFIG_FILE=' + etcd_config[idx], server_path],
                     stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    time.sleep(0.1)
    data = subprocess.run(['lsof', '-ti:'+str(endpoints[idx].split(':')[1])])
    if data.returncode != 0:
        print('start server {} failed'.format(idx))
        exit(1)


# start all servers
for i in range(len(endpoints)):
    start(i)

# Get IDs of nodes
ids = []
out = execute_etcdctl(endpoints, 'endpoint status')
for line in out.split('\n')[:-1]:
    ids.append(line.split(', ')[1])
if len(ids) != 6:
    print('retrieve IDs error: ' + ids)
    exit(1)

# Step 1: kill S5
print("Step 1: kill S5")
kill(5)
time.sleep(1)

# Step 2: make joint of 2 groups: S0-S2, S3-S5
print('Step 2: make joint of 2 groups: S0-S2, S3-S5')
execute_etcdctl([endpoints[0]],
                'member split {} --explict-leave'.format(','.join(ids[:3]) + ' ' + ','.join(ids[3:])))
time.sleep(1)

# Step 3: kill S3 and S4
print('Step 3: kill S3 and S4')
kill(3)
kill(4)
time.sleep(1)

# Step 4: let S0-S2 leave
print('Step 4: let S0-S2 leave')
execute_etcdctl([endpoints[0]],
                'member split {} --leave'.format(','.join(ids[:3]) + ' ' + ','.join(ids[3:])))
time.sleep(1)

# Step 5: split S0-S2 into S0-S1 & S2
print('Step 5: split S0-S2 into S0-S1 & S2')
execute_etcdctl([endpoints[0]],
                'member split {}'.format(','.join(ids[:2]) + ' ' + ids[2]))
time.sleep(1)

# Step 6: restart S5
print('Step 6: restart S5')
start(5)
time.sleep(1)

# Step 7: kill S0-S2
print('Step 7: kill S0-S2')
kill(0)
kill(1)
kill(2)
time.sleep(1)

# Step 8: restart S3 and S4
print('Step 8: restart S3 and S4')
start(3)
start(4)
time.sleep(1)

# Step 9: restart S0-S2
print('Step 9: restart S0-S2')
start(0)
start(1)
start(2)
time.sleep(1)

# kill all servers
for i in range(len(endpoints)):
    kill(i)
