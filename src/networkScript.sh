#!/bin/bash

CONTAINER_PID=$1

# Проверка и создание veth-пары
ip link add veth-host type veth peer name veth-container
ip link set veth-host up
ip addr add 192.168.1.123/24 dev veth-host
ip link set veth-container netns "$CONTAINER_PID"

# Настройка контейнера
nsenter --net=/proc/"$CONTAINER_PID"/ns/net ip link set veth-container up
nsenter --net=/proc/"$CONTAINER_PID"/ns/net ip addr add 192.168.1.124/24 dev veth-container
nsenter --net=/proc/"$CONTAINER_PID"/ns/net ip route add default via 192.168.1.123
# nsenter --net=/proc/"$CONTAINER_PID"/ns/net bash -c "echo 'nameserver 8.8.8.8' > /etc/resolv.conf"

# Настройка NAT на хосте
iptables -t nat -C POSTROUTING -s 192.168.1.0/24 -o enp3s0 -j MASQUERADE 2>/dev/null || iptables -t nat -A POSTROUTING -s 192.168.1.0/24 -o enp3s0 -j MASQUERADE
echo 1 > /proc/sys/net/ipv4/ip_forward

# Проверка и добавление правил FORWARD
iptables -C FORWARD -s 192.168.1.0/24 -o enp3s0 -j ACCEPT 2>/dev/null || iptables -A FORWARD -s 192.168.1.0/24 -o enp3s0 -j ACCEPT
iptables -C FORWARD -d 192.168.1.0/24 -m state --state RELATED,ESTABLISHED -j ACCEPT 2>/dev/null || iptables -A FORWARD -d 192.168.1.0/24 -m state --state RELATED,ESTABLISHED -j ACCEPT

echo "Network setup complete."
