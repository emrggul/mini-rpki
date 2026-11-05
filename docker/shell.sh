# Loaded by interactive /bin/sh through the POSIX ENV variable.
# Commands remain explicitly prefixed with mini-rpki.
help() {
    command mini-rpki --help "$@"
}

if [ -t 1 ]; then
    printf '\nMiniRPKI container shell\n'
    printf 'Type help for MiniRPKI commands. Prefix commands with mini-rpki.\n'
    printf 'Example: mini-rpki list\n\n'
fi
