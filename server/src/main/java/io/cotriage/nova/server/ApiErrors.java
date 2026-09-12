package io.cotriage.nova.server;

import static io.cotriage.nova.server.Values.*;

import jakarta.servlet.http.HttpServletRequest;
import java.util.List;
import org.springframework.dao.DataAccessException;
import org.springframework.http.ResponseEntity;
import org.springframework.http.converter.HttpMessageNotReadableException;
import org.springframework.web.bind.annotation.*;

@RestControllerAdvice
public class ApiErrors {
  @ExceptionHandler(ApiFailure.class)
  ResponseEntity<String> failure(ApiFailure error, HttpServletRequest request) {
    var body = obj("error", error.getMessage());
    String path = request.getRequestURI();
    if (path.startsWith("/api/v1/router/") && (error.status == 503 || error.status == 404)) {
      body.put("degraded", error.status == 503);
      if (error.status == 503
          && List.of(
                  "/api/v1/router/decisions",
                  "/api/v1/router/outbox",
                  "/api/v1/router/cloud-feedback")
              .contains(path)) {
        body.put("items", List.of());
        body.put("total", 0);
      }
    }
    return ManagementController.response(error.status, body);
  }

  @ExceptionHandler(HttpMessageNotReadableException.class)
  ResponseEntity<String> invalidBody() {
    return ManagementController.response(400, obj("error", "invalid JSON body"));
  }

  @ExceptionHandler(DataAccessException.class)
  ResponseEntity<String> database(DataAccessException error) {
    return ManagementController.response(
        500, obj("error", error.getMostSpecificCause().getMessage()));
  }
}
