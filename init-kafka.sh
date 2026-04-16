#!/bin/bash
set -e

echo "Creating topics..."

/opt/kafka/bin/kafka-topics.sh --bootstrap-server broker-1:19092 \
  --create --if-not-exists \
  --topic notifications \
  --partitions 3 \
  --replication-factor 3
/opt/kafka/bin/kafka-topics.sh --bootstrap-server broker-1:19092 \
  --create --if-not-exists \
  --topic notifications-dlq \
  --partitions 3 \
  --replication-factor 3

/opt/kafka/bin/kafka-topics.sh --bootstrap-server broker-2:19092 \
  --create --if-not-exists \
  --topic notifications \
  --partitions 3 \
  --replication-factor 3
/opt/kafka/bin/kafka-topics.sh --bootstrap-server broker-2:19092 \
  --create --if-not-exists \
  --topic notifications-dlq \
  --partitions 3 \
  --replication-factor 3

/opt/kafka/bin/kafka-topics.sh --bootstrap-server broker-3:19092 \
  --create --if-not-exists \
  --topic notifications \
  --partitions 3 \
  --replication-factor 3
/opt/kafka/bin/kafka-topics.sh --bootstrap-server broker-3:19092 \
  --create --if-not-exists \
  --topic notifications-dlq\
  --partitions 3 \
  --replication-factor 3

echo "Done."
