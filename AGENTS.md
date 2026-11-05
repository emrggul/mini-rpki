# MiniRPKI: guidance for future AI contributors

Read [docs/AI_KNOWLEDGE_BASE.md](docs/AI_KNOWLEDGE_BASE.md) before changing this project. The user-facing deployment instructions are in [CONTAINERLAB_GUIDE.md](CONTAINERLAB_GUIDE.md).

The active checkout is /home/emre/src/MiniRPKI in WSL Ubuntu-22.04. Do not recreate the removed Windows source copy.

Preserve the simple product: one executable, operator-defined VRPs, RTR v1, and a containerlab lab. The user prefers explicit one-line add/remove commands and has rejected scenario menus. The endpoint command is interface.

Protect existing VRPs and router configuration. Documentation work must not restart the lab or replace its data. The containerlab integration test clears the dataset and leaves demonstration records: inspect its script and arrange backup/restoration before running it against a user's lab.

Distinguish historical verification from tests executed in your current turn. Verify router validation states, not merely TCP connectivity. Never claim Nokia, Cisco, or Juniper interoperability without testing those devices.

Keep changes small, use the Go standard library where practical, and update these documents when behavior changes.
