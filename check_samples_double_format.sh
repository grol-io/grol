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
    ./grol -format -compact $file > /tmp/format3
    ./grol -format -compact /tmp/format3 > /tmp/format4
    diff -u /tmp/format3 /tmp/format4
    ./grol /tmp/format4 > /tmp/output3
    diff -u /tmp/output1 /tmp/output3
    echo "---done---"
done
