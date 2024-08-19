#! /bin/bash
set -e
export GOMEMLIMIT=1GiB
BIN="./grol -panic"
for file in "$@"; do
    echo "---testing double format for $file---"
    $BIN -format "$file" > /tmp/format1
    $BIN -format /tmp/format1 > /tmp/format2
    diff -u /tmp/format1 /tmp/format2
    $BIN "$file" > /tmp/output1
    $BIN /tmp/format2 > /tmp/output2
    diff -u /tmp/output1 /tmp/output2
    $BIN -format -compact "$file" > /tmp/format3
    $BIN -format -compact /tmp/format3 > /tmp/format4
    diff -u /tmp/format3 /tmp/format4
    $BIN /tmp/format4 > /tmp/output3
    diff -u /tmp/output1 /tmp/output3
    echo "---done---"
done
