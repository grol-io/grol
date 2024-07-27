#! /bin/bash
set -e
for file in "$@"; do
    echo "---testing double format for $file---"
    ./grol -format $file > /tmp/format1
    ./grol -format /tmp/format1 > /tmp/format2
    diff -u /tmp/format1 /tmp/format2
    ./grol $file > /tmp/output1
    ./grol /tmp/format2 > /tmp/output2
    diff -u /tmp/output1 /tmp/output2
    echo "---done---"
done
