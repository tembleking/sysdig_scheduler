FROM alpine
MAINTAINER "Federico Barcelona"
ADD . /root/go/src/github.com/tembleking/sysdig_scheduler
RUN apk --no-cache add go g++ git && \
    cd /root/go/src/github.com/tembleking/sysdig_scheduler && \
    go get -v && \ 
    mv /root/go/bin/sysdig_scheduler / && \
    cd / && \
    rm -rf /root/go && \
    apk del go g++ git
CMD /sysdig_scheduler
