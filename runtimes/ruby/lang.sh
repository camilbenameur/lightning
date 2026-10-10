# shellcheck shell=sh disable=SC2034 # sourced by common/run, which uses SOURCE_FILE
SOURCE_FILE=main.rb
# sync.rb makes $stdout unbuffered so output streams live and stays in order with $stderr.
run() { exec ruby -r/opt/lightning/sync.rb main.rb; }
