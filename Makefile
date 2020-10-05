start  :; ./ansible/run.sh ./configs/mainnet.ini ./ansible/main_start.yml
backup-keys :; ./ansible/run.sh ./configs/mainnet.ini ./ansible/main_backup_keystore.yml
debug :; ./ansible/run.sh ./configs/mainnet.ini ./ansible/main_debug.yml
restart :; ./ansible/run.sh ./configs/mainnet.ini ./ansible/main_restart_service.yml
show-addrs :; ./ansible/run.sh ./configs/mainnet.ini ./ansible/main_show_multiaddr.yml
update-binary :; ./ansible/run.sh ./configs/mainnet.ini ./ansible/main_update_binary.yml
start-monitor :; ./ansible/run.sh ./configs/mainnet.ini ./ansible/main_monitor.yml
