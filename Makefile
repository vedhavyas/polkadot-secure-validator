start  :; ./ansible/run.sh ${INVENTORY} ./ansible/main_start.yml
backup-keys :; ./ansible/run.sh ${INVENTORY} ./ansible/main_backup_keystore.yml
debug :; ./ansible/run.sh ${INVENTORY} ./ansible/main_debug.yml
restart :; ./ansible/run.sh ${INVENTORY} ./ansible/main_restart_service.yml
show-addrs :; ./ansible/run.sh ${INVENTORY} ./ansible/main_show_multiaddr.yml
update-binary :; ./ansible/run.sh ${INVENTORY} ./ansible/main_update_binary.yml
start-monitor :; ./ansible/run.sh ${INVENTORY} ./ansible/main_monitor.yml
