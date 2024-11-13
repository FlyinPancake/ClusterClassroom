[private]
build_tape file:
    vhs {{ file }}

build_install_tape: (build_tape "docs/install-controller.tape")