# game-images

Game server docker images for the following games:

* [alt:V](altv/README.md)
* [ARK: Survival Evolved](ark/README.md)
* [Arma III](arma3/README.md)
* [Counter Strike: Global Offensive](csgo/README.md)
* [Garry's Mod](gmod/README.md)
* [Rust](rust/README.md)
* [SCP: Secret Laboratory](scpsl/README.md)
* [Valheim](valheim/README.md)

See the README of individual games for more information.

To run Minecraft servers, the following image is recommended: https://github.com/itzg/docker-minecraft-server.

These images are used for [IPS Hosting](https://www.ips-hosting.com/), a german game server hosting provider. However, feel free to use them for your own needs.

## Building and Versioning
On every push to the main branch, all images that were changed are automatically rebuilt and deployed to Docker Hub using the following tags:
* The image version (e.g. v1, v2, ...)
* latest

In addition, each image is rebuilt and deployed weekly at Sunday 00:00 to keep up with latest security patches.

The image version is configured as an env variable in the workflow file of the game in the `.github/workflows/` directory and should be raised whenever a breaking change is introduced to a game image.

PRs are welcome!
