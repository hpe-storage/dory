FROM golang as builder
RUN git clone https://github.com/hpe-storage/dory
WORKDIR dory
RUN make gettools && \
    make vendor && \
    make doryd 

FROM alpine:latest
COPY --from=builder /go/dory/bin/doryd /usr/local/bin/doryd
ENTRYPOINT [ "doryd" ]
CMD [ "/etc/kubernetes/admin.conf", "dev.hpe.com" ]
