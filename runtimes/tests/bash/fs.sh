for path in /etc/lightning /workspace/../lightning /workspace/lightning /tmp/lightning; do
  if (: >"$path") 2>/dev/null; then
    echo "$path: ok"
  else
    echo "$path: error"
  fi
done
