# shellcheck shell=sh disable=SC2034 # sourced by common/run, which uses SOURCE_FILE
SOURCE_FILE=main.php
run() { exec php main.php; }
