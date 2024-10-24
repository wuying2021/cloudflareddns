### CloudflareDDNS
使用golang编写的 CloudflareDDNS、支持使用APITOKEN进行dns更新

### 使用
```
docker run -d \
--network host \
-e APITOKEN=XXXXXXXXXXXXXXXXXXXXXXX \
-e DOMAIN=example.com \
-e PREFIX=prefix \
ghcr.io/wuying2021/cloudflareddns
```
