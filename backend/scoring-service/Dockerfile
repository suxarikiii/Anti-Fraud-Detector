FROM gradle:8.10-jdk17-alpine AS builder

WORKDIR /src

COPY build.gradle.kts settings.gradle.kts ./
COPY gradle/ ./gradle/
RUN gradle dependencies --no-daemon || true

COPY src/ ./src/
RUN gradle bootJar --no-daemon

FROM eclipse-temurin:17-jre-alpine

RUN apk add --no-cache tzdata

WORKDIR /app
COPY --from=builder /src/build/libs/*.jar app.jar

EXPOSE 8083

CMD ["java", "-jar", "app.jar"]
