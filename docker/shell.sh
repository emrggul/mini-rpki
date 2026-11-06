# Loaded by interactive /bin/sh through the POSIX ENV variable.
# Commands remain explicitly prefixed with mini-rpki.
help() {
    command mini-rpki --help "$@"
}

# Containerlab's attached shell is often root, while the MiniRPKI server runs
# as UID 10001.  Generate credentials as that service identity so its private
# key directory remains usable when the server starts.
mini-rpki() {
    if [ "${1:-}" = "tls" ] && [ "$(id -u)" = "0" ]; then
        command su-exec 10001:10001 mini-rpki "$@"
    else
        command mini-rpki "$@"
    fi
}

if [ -t 1 ]; then
    printf '\nMiniRPKI container shell\n'
    printf 'Type help for MiniRPKI commands. Prefix commands with mini-rpki.\n'
    printf 'Example: mini-rpki list\n\n'
fi
