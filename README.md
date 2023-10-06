### Start relay (chisel server)
```
chisel server -v --auth user:pass --port 8081 --reverse
```

### Connect from target (machine you need to connect to)
```
tunnel -auth user:pass -srv http://localhost:8081 --id E4880901-D64A-4888-8C15-D4F96BC19440 --sftp host 
```

### Connect to target from local
```
tunnel --auth user:pass --srv http://localhost:8081 --id E4880901-D64A-4888-8C15-D4F96BC19440 --sftp connect
```
