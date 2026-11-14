#!/bin/bash
# Outdated server script with conflicts and legacy flags
java -Xms2G -Xmx16G \
  -XX:+UseG1GC \
  -XX:+UseParallelGC \
  -XX:+CMSIncrementalMode \
  -jar server.jar nogui
