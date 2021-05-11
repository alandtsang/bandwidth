# bandwidth
cni bandwidth for cilium

# Test
OS: Ubuntu 18.04
Kernel: 5.4.0

Pod A and Pod B install `tc` and `iperf3`.

Pod A annotations add:
```yaml
annotations:
  kubernetes.io/ingress-bandwidth: 10M
  kubernetes.io/egress-bandwidth: 10M
```

Pod ingress bandwidth limit test：

Pod A:
```
# iperf3 -s
```

Pod B:
```
# iperf3 -c 10.0.0.51 -b 15M
```

Pod A receiver result：
```
-----------------------------------------------------------
Server listening on 5201
-----------------------------------------------------------
Accepted connection from 10.0.0.208, port 58454
[  5] local 10.0.0.51 port 5201 connected to 10.0.0.208 port 58456
[ ID] Interval           Transfer     Bitrate
[  5]   0.00-1.00   sec  1.14 MBytes  9.59 Mbits/sec                  
[  5]   1.00-2.00   sec  1.14 MBytes  9.52 Mbits/sec                  
[  5]   2.00-3.00   sec  1.14 MBytes  9.57 Mbits/sec                  
[  5]   3.00-4.00   sec  1.14 MBytes  9.56 Mbits/sec                  
[  5]   4.00-5.00   sec  1.14 MBytes  9.58 Mbits/sec                  
[  5]   5.00-6.00   sec  1.14 MBytes  9.57 Mbits/sec                  
[  5]   6.00-7.00   sec  1.14 MBytes  9.56 Mbits/sec                  
[  5]   7.00-8.00   sec  1.14 MBytes  9.57 Mbits/sec                  
[  5]   8.00-9.00   sec  1.14 MBytes  9.55 Mbits/sec                  
[  5]   9.00-10.00  sec  1.14 MBytes  9.57 Mbits/sec                  
[  5]  10.00-10.01  sec  5.66 KBytes  7.70 Mbits/sec                  
- - - - - - - - - - - - - - - - - - - - - - - - -
[ ID] Interval           Transfer     Bitrate
[  5]   0.00-10.01  sec  11.4 MBytes  9.56 Mbits/sec           receiver
```

Pod egress bandwidth limit test：

Pod A:
```
# iperf3 -s
```

Pod B:
```
# iperf3 -c 10.0.0.51 -R -b 15M
```

Pod B receiver result：
```
Connecting to host 10.0.0.51, port 5201
Reverse mode, remote host 10.0.0.51 is sending
[  4] local 10.0.0.208 port 59376 connected to 10.0.0.51 port 5201
[ ID] Interval           Transfer     Bandwidth
[  4]   0.00-1.00   sec  1.14 MBytes  9.58 Mbits/sec                  
[  4]   1.00-2.00   sec  1.14 MBytes  9.55 Mbits/sec                  
[  4]   2.00-3.00   sec  1.14 MBytes  9.55 Mbits/sec                  
[  4]   3.00-4.00   sec  1.14 MBytes  9.57 Mbits/sec                  
[  4]   4.00-5.00   sec  1.14 MBytes  9.55 Mbits/sec                  
[  4]   5.00-6.00   sec  1.14 MBytes  9.58 Mbits/sec                  
[  4]   6.00-7.00   sec  1.14 MBytes  9.57 Mbits/sec                  
[  4]   7.00-8.00   sec  1.14 MBytes  9.54 Mbits/sec                  
[  4]   8.00-9.00   sec  1.14 MBytes  9.59 Mbits/sec                  
[  4]   9.00-10.00  sec  1.14 MBytes  9.56 Mbits/sec                  
- - - - - - - - - - - - - - - - - - - - - - - - -
[ ID] Interval           Transfer     Bandwidth       Retr
[  4]   0.00-10.00  sec  12.2 MBytes  10.3 Mbits/sec    0             sender
[  4]   0.00-10.00  sec  11.5 MBytes  9.67 Mbits/sec                  receiver
```
