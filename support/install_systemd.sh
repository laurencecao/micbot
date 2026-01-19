#!/bin/bash

cp supervisord.service /etc/systemd/system/
systemctl daemon-reload

sudo systemctl enable --now supervisord
sudo systemctl status supervisord

