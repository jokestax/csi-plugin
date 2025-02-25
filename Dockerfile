FROM alpine

COPY ./csi-plugin /usr/local/bin/csi-plugin
RUN chmod +x /usr/local/bin/csi-plugin

ENTRYPOINT ["/usr/local/bin/csi-plugin"]
