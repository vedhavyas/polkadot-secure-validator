# Polkadot Secure Validator Setup
This repo a fork of [Polkadot secure validator](https://github.com/w3f/polkadot-secure-validator) describes a potential setup for a Polkadot validator that aims to
prevent some types of potential attacks at the TCP layer and below.
The [Application Layer](#application-layer) describes in more detail.

## Usage
Setup Debian-based machines yourself, which only need basic SSH access and 
configure those in an inventory. The Ansible scripts will setup the entire 
[Application Layer](#application-layer). Enable port 80 on the instance to receive connections from other peers.

Use `make` to run any ansible commands. 
- `start`: Starts the new validator. This is an idempotent call.
- `backup-keys`: Backups keys to local machine at the location defined in the config. Folder must be present.
- `debug`: prints the validator logs to command line
- `restart`: restarts the validator
- `show-addrs`: displays validator multi address
- `update-binary`: updates the binary
- `start-monitor`: starts the monitoring service


## Structure
The secure validator setup composed of a validator that run with a local
instance of NGINX as a reverse TCP proxy in front of them. The validators are instructed to:
* advertise themselves with the public IP of the node and the port where the
reverse proxy is listening.
* bind to the localhost interface, so that they only allow incoming connections from the
proxy.

The setup also configures a firewall in which the default p2p port is closed for
incoming connections and only the proxy port is open.

## Application Layer

This is done through the ansible playbook and polkadot-validator role located at
[ansible](/ansible), basically the role performs these actions:

* Software firewall setup, for the validator we only allow the proxy, SSH and, if
enabled, node-exporter ports.
* Configure journald to tune log storage.
* Create polkadot user and group.
* Configure NGINX proxy
* Setup polkadot service, including binary download.
* Polkadot session management, create session keys if they are not present.
* Setup node-exporter if the configuration includes it.
