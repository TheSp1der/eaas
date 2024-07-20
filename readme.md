# EaaS

## Table of Contents

1. [Overview](#overview)
1. [Stage](#stage)
1. [Sources](#sources)

## Overview

This project aims to provide entropy to systems via a server/client system in go. The server will need to have a trusted source to generate entropy and enough entropy to properly serve the clients that request entropy from it.

## Stage

This project is currently in development and should not be considered for production use.

## Sources

The default source is from the host's /dev/random source. This allows you to use a system that is pre-configured with any entropy sources. An alternative source built into the server is the usage of the [OneRNG](https://onerng.info/) an open-source/open-hardware random number generator.