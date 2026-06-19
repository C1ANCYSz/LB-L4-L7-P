# 1. Track new connections on port 8080
sudo iptables -A INPUT -p tcp --dport 8080 -m state --state NEW -m recent --set

# 2. If an IP exceeds 100 new connections in 60 seconds, DROP the packets
sudo iptables -A INPUT -p tcp --dport 8080 -m state --state NEW -m recent --update --seconds 60 --hitcount 100 -j DROP
