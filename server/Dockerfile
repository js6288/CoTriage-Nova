FROM eclipse-temurin:25-jdk AS build
WORKDIR /build
COPY . .
RUN --mount=type=cache,id=cotriage-nova-maven,target=/root/.m2,sharing=locked chmod +x mvnw && ./mvnw -B -ntp -pl server -am -DskipTests package

FROM eclipse-temurin:25-jre
WORKDIR /app
ENV JAVA_TOOL_OPTIONS="-Xms64m -Xmx256m"
COPY --from=build --chown=10001:10001 /build/server/target/server-0.1.0-SNAPSHOT.jar /app/service.jar
USER 10001:10001
EXPOSE 8080
ENTRYPOINT ["java","-jar","/app/service.jar"]
