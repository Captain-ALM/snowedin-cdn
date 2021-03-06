# SnoweDiN
### Snow Services CDN

[![Build Status](https://ci.mrmelon54.xyz/api/badges/snow/snowedin/status.svg)](https://ci.mrmelon54.xyz/snow/snowedin)

This allows for content to be served off different zones with limits per IP address for concurrent connections, requests in an interval and bandwidth. 
There is also configuration for backends (And can be extended by building with more backends). 
This also supports cache processing using headers and 304 redirects; download hinting headers are also supported.
Supports range requests and partial content responses.

The use of DELETE is possible to tell the zone to clear cache in its backend and itself; GET, OPTIONS and HEAD are also supported.

Maintainer: 
[Captain ALM](https://code.mrmelon54.xyz/alfred)

License: 
[ISC Based License](https://code.mrmelon54.xyz/snow/snowedin/src/branch/master/LICENSE.md)

Example configuration: 
[config.example.yml](https://code.mrmelon54.xyz/snow/snowedin/src/branch/master/config.example.yml) 
The configuration must by placed in a .data sub-directory from the executable. A .env file must also be generated (Can be empty).

### TODO:

- Add global limits per zone and for the entire CDN.
- Add the API server support.
- Add PUT support per zone for whitelisted IPs.
- Add a backend that sends requests to another server.
- Add a backend that sends requests to another server and caches them on the filesystem.
- Turn zone into a middleware provider.
- Support authentication.