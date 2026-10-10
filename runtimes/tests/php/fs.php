<?php

foreach (["/etc/lightning", "/workspace/../lightning", "/workspace/lightning", "/tmp/lightning"] as $path) {
    $result = @file_put_contents($path, "") === false ? "error" : "ok";
    echo "$path: $result\n";
}
