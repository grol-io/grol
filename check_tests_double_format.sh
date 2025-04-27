#! /bin/bash
set -e
export GOMEMLIMIT=1GiB
BIN="./grol -panic -shared-state -no-progress"
echo "---testing double format for tests ---"
$BIN -format tests/*.gr > /tmp/format1
$BIN -format /tmp/format1 > /tmp/format2
diff -u /tmp/format1 /tmp/format2
$BIN -eval=false tests/*.gr > /tmp/output1
$BIN /tmp/format2 > /tmp/output2
diff -u /tmp/output1 /tmp/output2
$BIN -format -compact tests/*.gr > /tmp/format3
$BIN -format -compact /tmp/format3 > /tmp/format4
diff -u /tmp/format3 /tmp/format4
$BIN /tmp/format4 > /tmp/output3
diff -u /tmp/output1 /tmp/output3
echo "---done---"
