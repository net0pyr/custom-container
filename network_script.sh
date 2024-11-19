#!/bin/bash

# IP-адреса для настройки
HOST_IP="192.168.1.123"
CONTAINER_IP="192.168.1.124"
SUBNET="192.168.1.0/24"
CONTAINER_PID=$1

# Проверка занятости IP-адресов на хосте
if ip addr | grep -q "$HOST_IP"; then
    echo "Error: IP address $HOST_IP is already in use on the host."
    exit 1
fi

if ip addr | grep -q "$CONTAINER_IP"; then
    echo "Error: IP address $CONTAINER_IP is already in use on the host."
    exit 1
fi

# Проверка доступности IP-адресов в сети
if ping -c 1 "$HOST_IP" &> /dev/null; then
    echo "Error: IP address $HOST_IP is already in use in the network."
    exit 1
fi

if ping -c 1 "$CONTAINER_IP" &> /dev/null; then
    echo "Error: IP address $CONTAINER_IP is already in use in the network."
    exit 1
fi

# Создание пары интерфейсов veth
ip link add veth-host type veth peer name veth-container

# Поднятие интерфейса на хосте
ip link set veth-host up
ip addr add "$HOST_IP/24" dev veth-host

# Перемещение интерфейса контейнера в сетевой namespace контейнера
ip link set veth-container netns "$CONTAINER_PID"

# Настройка интерфейса в контейнере
nsenter --net=/proc/"$CONTAINER_PID"/ns/net ip link set veth-container up
nsenter --net=/proc/"$CONTAINER_PID"/ns/net ip addr add "$CONTAINER_IP/24" dev veth-container
nsenter --net=/proc/"$CONTAINER_PID"/ns/net ip route add default via "$HOST_IP"

# Настройка маршрутизации и NAT на хосте
iptables -t nat -A POSTROUTING -s "$SUBNET" -j MASQUERADE
sysctl -w net.ipv4.ip_forward=1

echo "Network setup complete."