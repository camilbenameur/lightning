for path in ["/etc/lightning", "/workspace/../lightning", "/workspace/lightning", "/tmp/lightning"]:
    try:
        open(path, "w").close()
        print(f"{path}: ok")
    except OSError:
        print(f"{path}: error")
