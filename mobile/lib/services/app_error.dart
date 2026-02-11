import 'dart:io';

import 'package:dio/dio.dart';

/// Translates raw exceptions (Dio, Socket, etc.) into user-friendly messages.
String formatError(Object error) {
  if (error is AppException) {
    return error.userMessage;
  }

  if (error is DioException) {
    return _dioMessage(error);
  }

  if (error is SocketException) {
    return 'Unable to reach the server. Check your internet connection.';
  }

  if (error is HttpException) {
    return 'A network error occurred. Please try again.';
  }

  if (error is FormatException) {
    return 'Received an unexpected response from the server.';
  }

  final msg = error.toString();
  // Avoid dumping raw exception class names at the user.
  if (msg.contains('SocketException') || msg.contains('Connection refused')) {
    return 'Unable to reach the server. Check your internet connection.';
  }

  return 'Something went wrong. Please try again.';
}

/// Categorised application exception with a human-readable message.
class AppException implements Exception {
  final String userMessage;
  final Object? cause;

  const AppException(this.userMessage, {this.cause});

  @override
  String toString() => userMessage;

  /// Create from a Dio error with context-appropriate messaging.
  factory AppException.fromDio(DioException error) {
    return AppException(_dioMessage(error), cause: error);
  }
}

String _dioMessage(DioException error) {
  switch (error.type) {
    case DioExceptionType.connectionTimeout:
    case DioExceptionType.sendTimeout:
    case DioExceptionType.receiveTimeout:
      return 'Connection timed out. Check your network and try again.';

    case DioExceptionType.connectionError:
      return 'Unable to reach the server. Check your internet connection.';

    case DioExceptionType.badResponse:
      return _httpStatusMessage(error.response?.statusCode, error.response);

    case DioExceptionType.cancel:
      return 'Request was cancelled.';

    case DioExceptionType.badCertificate:
      return 'Server certificate is invalid. Contact your administrator.';

    case DioExceptionType.unknown:
      if (error.error is SocketException) {
        return 'Unable to reach the server. Check your internet connection.';
      }
      return 'A network error occurred. Please try again.';
  }
}

String _httpStatusMessage(int? statusCode, Response? response) {
  // Try to extract an error message from the API envelope.
  final apiError = _extractApiError(response);

  switch (statusCode) {
    case 401:
      return apiError ?? 'Authentication failed. Please sign in again.';
    case 403:
      return apiError ?? 'You do not have permission to perform this action.';
    case 404:
      return apiError ?? 'The requested resource was not found.';
    case 409:
      return apiError ?? 'A conflict occurred. Please try again.';
    case 422:
      return apiError ?? 'Invalid data submitted. Please check your input.';
    case 429:
      return 'Too many requests. Please wait a moment and try again.';
    case 500:
    case 502:
    case 503:
      return 'The server is experiencing issues. Please try again later.';
    default:
      if (statusCode != null && statusCode >= 400) {
        return apiError ?? 'Server error ($statusCode). Please try again.';
      }
      return 'An unexpected error occurred.';
  }
}

/// Extract the error message from the FlowPBX API envelope: { "error": "..." }
String? _extractApiError(Response? response) {
  final data = response?.data;
  if (data is Map<String, dynamic> && data.containsKey('error')) {
    final error = data['error'];
    if (error is String && error.isNotEmpty) {
      return error;
    }
  }
  return null;
}
