FROM ubuntu
RUN apt-get update
ENV PORT 80
COPY api /opt/
EXPOSE 80
CMD ["/opt/api"]
