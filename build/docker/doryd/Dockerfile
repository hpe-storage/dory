FROM alpine:latest
ADD [ "bin/doryd", "/usr/local/bin/doryd" ]
ENTRYPOINT [ "doryd" ]
CMD [ "/etc/kubernetes/admin.conf", "dev.hpe.com" ]
