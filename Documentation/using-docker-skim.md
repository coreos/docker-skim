# Using docker-skim

**Note: running the docker daemon with this tool is not supported and may break with no notice**.

## Using docker-skim to run docker 1.12

On a Container Linux machine, setup the following systemd unit for docker.
Override the existing unit by placing it at the path
`/etc/systemd/system/docker.service`.

If this is done on a running machine, rather than during provisioning via Ignition, first stop the existing docker and containerd units.

```ini
[Unit]
Description=Docker Application Container Engine
Documentation=http://docs.docker.com
After=docker.socket
Requires=docker.socket

[Service]
Type=simple

ExecStart=/usr/bin/rkt run --dns=host --interactive \
  --insecure-options=image \
  --stage1-name=users.developer.core-os.net/skim/stage1-skim:0.0.1 \
  users.developer.core-os.net/skim/docker:1.12.6_coreos.0 \
  --exec=/usr/lib/coreos/dockerd -- \
  --host=fd:// $DOCKER_OPTS $DOCKER_CGROUPS $DOCKER_OPT_BIP $DOCKER_OPT_MTU $DOCKER_OPT_IPMASQ

ExecReload=/bin/kill -s HUP $MAINPID
LimitNOFILE=1048576
LimitNPROC=infinity
LimitCORE=infinity
TasksMax=infinity
TimeoutStartSec=0
# set delegate yes so that systemd does not reset the cgroups of docker containers
Delegate=yes

[Install]
WantedBy=multi-user.target
```

Afterwards, run `systemctl daemon-reload` and `systemctl start docker` per usual.

### The docker CLI

The `docker` command will, by default, be the client version shipped in the host operating system. Generally, this CLI will work with older docker daemon versions.

In order to use a matching `docker` cli version, the following script can be used as the docker command (e.g. placed in `/opt/bin/docker` and added to a user's path):

```sh
#!/bin/bash

[[ -f /etc/coreos/docker.conf ]] && source /etc/coreos/docker.conf

# The docker command is only accessible if the service is running

if [ ! -x "/usr/bin/rkt" ]; then
	echo "rkt is not installed"
	exit 254
fi

RKTPODBASE="/var/lib/rkt/pods/run"
STAGE2BRIDGE="/stage1/rootfs/opt/stage2/"

ROUT=`rkt list --format=json`

if [ "x${ROUT}" == "xnull" ]; then
	echo "Docker cannot be started"
	exit 254
fi

UUIDNAME=`echo ${ROUT}|jq -r '.[]|select(.state == "running" and (.app_names[]|contains("docker")))|.name'`

DOCKER="${RKTPODBASE}/${UUIDNAME}/${STAGE2BRIDGE}/docker/rootfs/usr/bin/docker"
exec ${DOCKER} "$@"
```
