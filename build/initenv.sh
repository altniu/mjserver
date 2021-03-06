#!/bin/sh

REMOTE=129.204.58.232

echo "============================="
echo "==== initialization env"
echo "============================="
ssh -i txy_root root@$REMOTE <<EOF

echo "==>> install supervisord"
yum install -y python-setuptools
easy_install supervisor

echo "==>> init supervisord config"
mkdir -p /etc/supervisor.d/
echo_supervisord_conf > /etc/supervisord.conf
echo "[include]" >> /etc/supervisord.conf
echo "files = /etc/supervisor.d/*.conf" >> /etc/supervisord.conf

echo "==>> prepare game logic server env"
mkdir -p /opt/triple/
mkdir -p /opt/triple/logs

echo "==>> supervisor configuration"
echo "[program:triple]" >> /etc/supervisor.d/triple.conf
echo "command=/opt/triple/tripled -c configs/config.prod.toml" >> /etc/supervisor.d/triple.conf
echo "directory=/opt/triple" >> /etc/supervisor.d/triple.conf
echo "stdout_logfile=/opt/triple/logs/triple.log" >> /etc/supervisor.d/triple.conf
echo "stderr_logfile=/opt/triple/logs/triple.log" >> /etc/supervisor.d/triple.conf
echo "stopsignal=INT" >> /etc/supervisor.d/triple.conf
EOF

echo "done"