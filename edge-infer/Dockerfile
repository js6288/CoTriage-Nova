FROM eclipse-temurin:25-jdk AS build
WORKDIR /build
COPY . .
RUN --mount=type=cache,id=cotriage-nova-maven,target=/root/.m2,sharing=locked chmod +x mvnw && ./mvnw -B -ntp -pl edge-infer -am -DskipTests package

FROM eclipse-temurin:25-jre
WORKDIR /app
ENV JAVA_TOOL_OPTIONS="-Xms64m -Xmx256m"
COPY --from=build --chown=10001:10001 /build/edge-infer/target/edge-infer-0.1.0-SNAPSHOT.jar /app/service.jar
COPY --from=build --chown=10001:10001 /build/edge-infer/config /app/config
USER 10001:10001
ENTRYPOINT ["java","-jar","/app/service.jar"]
