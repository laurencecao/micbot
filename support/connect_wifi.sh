
nmcli device wifi connect "medishare" password "medi123456"
nmcli connection modify "medishare" connection.autoconnect yes
systemctl restart NetworkManager