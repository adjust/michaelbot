FROM alpine:3.3
MAINTAINER Bubunyo Nyavor <samuel.bubunyo.nyavor@adjust.com>

RUN apk add -U ca-certificates

EXPOSE 8081
ADD michael /bin/server

CMD ["/bin/server", "-h", "0.0.0.0", "-p", "8081"]
