#!/bin/bash

function handle_error() {
  if (( $? )) ; then
    echo -e "[\e[31mERROR\e[39m]"
    echo -e >&2 "CAUSE:\n $1"
    exit 1
  else
    echo -e "[\e[32mOK\e[39m]"
  fi
}

echo "Sudo password for remote servers:"
read -s SUDO_PW

echo -n ">> Testing Ansible availability... "
out=$((ansible --version) 2>&1)
handle_error "$out"

echo -n ">> Finding validator hosts... "
out=$((ansible -i inventory.ini validator --list-hosts) 2>/dev/null)
if [[ $out == *"hosts (0)"* ]]; then
  out="No hosts found, exiting..."
  (exit 1)
  handle_error "$out"
else
  echo -e "[\e[32mOK\e[39m]"
  echo "$out"
fi

echo -n ">> Testing connectivity to nodes... "
out=$((ansible all -i inventory.ini -m ping --become --extra-vars "ansible_become_pass='$SUDO_PW'") 2>&1)
handle_error "$out"

echo ">> Executing Ansible Playbook..."

ansible-playbook -i inventory.ini main.yml --become --extra-vars "ansible_become_pass='$SUDO_PW'"

echo ">> Done!"
