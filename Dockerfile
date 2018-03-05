FROM alpine
MAINTAINER "Federico Barcelona"
RUN apk --no-cache add go g++ git && \
    go get -t -v github.com/tembleking/sysdig_scheduler && \
    go build github.com/tembleking/sysdig_scheduler && \ 
    rm -rf /root/go && \
    apk del go g++ git
CMD /sysdig_scheduler