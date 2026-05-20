#!/bin/bash
set -e

echo "Creating topics..."

/opt/kafka/bin/kafka-topics.sh --bootstrap-server broker-1:19092 \
  --create --if-not-exists \
  --topic link.raw-updates \
  --partitions 3 \
  --replication-factor 3
/opt/kafka/bin/kafka-topics.sh --bootstrap-server broker-1:19092 \
  --create --if-not-exists \
  --topic link.processed-updates \
  --partitions 3 \
  --replication-factor 3
/opt/kafka/bin/kafka-topics.sh --bootstrap-server broker-1:19092 \
  --create --if-not-exists \
  --topic link.raw-updates-dlq \
  --partitions 3 \
  --replication-factor 3

/opt/kafka/bin/kafka-topics.sh --bootstrap-server broker-2:19092 \
  --create --if-not-exists \
  --topic link.raw-updates \
  --partitions 3 \
  --replication-factor 3
/opt/kafka/bin/kafka-topics.sh --bootstrap-server broker-1:19092 \
  --create --if-not-exists \
  --topic link.processed-updates \
  --partitions 3 \
  --replication-factor 3
/opt/kafka/bin/kafka-topics.sh --bootstrap-server broker-2:19092 \
  --create --if-not-exists \
  --topic link.raw-updates-dlq \
  --partitions 3 \
  --replication-factor 3

/opt/kafka/bin/kafka-topics.sh --bootstrap-server broker-3:19092 \
  --create --if-not-exists \
  --topic link.raw-updates \
  --partitions 3 \
  --replication-factor 3
/opt/kafka/bin/kafka-topics.sh --bootstrap-server broker-1:19092 \
  --create --if-not-exists \
  --topic link.processed-updates \
  --partitions 3 \
  --replication-factor 3
/opt/kafka/bin/kafka-topics.sh --bootstrap-server broker-3:19092 \
  --create --if-not-exists \
  --topic link.raw-updates-dlq\
  --partitions 3 \
  --replication-factor 3

echo "Done."
