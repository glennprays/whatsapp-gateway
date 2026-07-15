# Overview

## Introduction

Whatsapp Gateway is an open-source, self-hosted service designed to provide a production-grade integration layer between backend systems and the WhatsApp network.

The gateway abstracts the operational complexity of WhatsApp Web multi-device communication and exposes a controlled REST API and webhook interface for backend services. It is intended for backend engineers and solution architects who require a reliable, maintainable, and secure WhatsApp integration component.

This project is open for external contributions. All contributions are subject to pull request review to maintain architectural consistency and production standards.

## Project Scope

Whatsapp Gateway is designed to:

- Provide a stateless HTTP interface for sending messages
- Handle WhatsApp session lifecycle management
- Support message delivery tracking
- Process inbound messages and media via webhooks
- Enforce rate limiting
- Optionally queue outbound messages using RabbitMQ
- Persist session and message data using a configurable database backend

The gateway is intended to be deployed behind a proper backend service and should not function as a direct client-facing application.

## Non-Affiliation Notice

This project is **not affiliated with, endorsed by, or sponsored by WhatsApp or Meta**.

It utilizes the `whatsmeow` library, which implements the WhatsApp Web multi-device protocol. Usage of this gateway is subject to WhatsApp’s terms of service and any applicable policies.

Users are responsible for ensuring compliance with applicable platform policies and regulations.

## Supported Environments

The gateway is primarily distributed as a Docker container and is designed to be platform-agnostic.

Supported environments include:

- Linux (recommended for production)
- macOS (development)
- Windows Subsystem for Linux (development)
- Native binary builds for supported Go platforms

For production deployments, Linux-based container environments are strongly recommended.

## Design Philosophy

The system follows these core principles:

- **Separation of Concerns**: Business logic belongs in your backend; WhatsApp state management belongs in the gateway.
- **Stateless API Layer**: Backend services should not manage WhatsApp session complexity.
- **Operational Transparency**: Health status and device state must be observable.
- **Security by Design**: JWT authentication and HMAC-based webhook validation are enforced.
- **Queue-Driven Scalability (Optional)**: RabbitMQ integration allows controlled message throughput.
- **Production Readiness**: Clear boundaries, predictable behavior, and explicit failure handling.

## High-Level Capability Summary

- Send text messages
- Send media messages (image support)
- Receive inbound messages and media via webhook
- Delivery status tracking
- Rate limiting with optional queue-based processing
- RabbitMQ integration for outbound message control
- PostgreSQL or SQLite database support
- Health and WhatsApp session status endpoints

## Deployment Boundary

Whatsapp Gateway must be wrapped by a backend service. It is recommended to:

- Deploy within a private network
- Protect with JWT authentication
- Validate webhook payloads using HMAC signature verification

Although it can be exposed externally with proper authentication, internal network deployment is considered best practice.
