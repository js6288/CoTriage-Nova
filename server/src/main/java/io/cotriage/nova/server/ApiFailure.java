package io.cotriage.nova.server;

final class ApiFailure extends RuntimeException {
  final int status;

  ApiFailure(int status, String message) {
    super(message);
    this.status = status;
  }
}
