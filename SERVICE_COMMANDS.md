# AV1 Daemon Service Management

## Basic Service Commands

### Start the service
```bash
sudo systemctl start av1d
```

### Stop the service
```bash
sudo systemctl stop av1d
```

### Restart the service
```bash
sudo systemctl restart av1d
```

### Reload configuration (without stopping)
```bash
sudo systemctl reload av1d
```
Note: This only works if the service supports reload. For av1d, use `restart` instead.

### Check service status
```bash
sudo systemctl status av1d
```

### Enable auto-start on boot
```bash
sudo systemctl enable av1d
```

### Disable auto-start on boot
```bash
sudo systemctl disable av1d
```

## Viewing Logs

### View recent logs
```bash
sudo journalctl -u av1d
```

### Follow logs in real-time
```bash
sudo journalctl -u av1d -f
```

### View last 100 lines
```bash
sudo journalctl -u av1d -n 100
```

### View logs since today
```bash
sudo journalctl -u av1d --since today
```

## After Configuration Changes

If you modify `/etc/av1qsvd/config.json`, restart the service:

```bash
sudo systemctl restart av1d
```

## Troubleshooting

### Service won't start
1. Check status for errors:
   ```bash
   sudo systemctl status av1d
   ```

2. Check logs:
   ```bash
   sudo journalctl -u av1d -n 50
   ```

3. Verify configuration file exists and is valid:
   ```bash
   sudo cat /etc/av1qsvd/config.json
   ```

4. Check file permissions:
   ```bash
   ls -la /etc/av1qsvd/config.json
   ls -la /var/lib/av1qsvd/
   ```

### Service keeps restarting
Check logs to see why it's crashing:
```bash
sudo journalctl -u av1d -n 100 --no-pager
```

### Verify service is running
```bash
# Check if process is running
ps aux | grep av1d

# Check systemd status
systemctl is-active av1d
```

