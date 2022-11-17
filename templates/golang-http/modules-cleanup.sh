#!/usr/bin/env sh

set -e

GO111MODULE=$(go env GO111MODULE)

[ -z "$DEBUG" ] && DEBUG=0

# move_vendor will copy the function's vendor folder,
# if it exists.
move_vendor() {
    if [ ! -d ./function/vendor ]; then
        echo "vendor not found"
        return
    fi

    echo "moving function vendor"
    mv -f ./function/vendor .
}


# cleanup_gomod will move the function's go module
cleanup_gomod() {

    # Nothing to do when modules is explicitly off
    # the z prefix protects against any SH wonkiness
    # see https://stackoverflow.com/a/18264223
    if [ "z$GO111MODULE" = "zoff" ]; then
        echo "modules disabled, skipping go.mod cleanup"
        return;
    fi

    if [ ! -f ./function/go.mod ]; then
        echo "module not initialized, skipping go.mod cleanup"
        return;
    fi

    echo "cleaning up go.mod"

    # Copy the user's go.mod
    mv -f ./function/go.mod .
    mv -f ./function/go.sum .

    # Clean up the go.mod

    # Cleanup any sub-module replacements.
    # This requires modifying any replace that points to "./*",
    # the user has will use this to reference sub-modules instead
    # of sub-packages, which we cleanup below.
    echo "cleanup local replace statements"
    # 1. Replace references to the local folder with `./function`
    sed -i 's/=> \.\//=> \.\/function\//' go.mod


    # Remove any references to the handler/function module.
    # It is ok to just remove it because we will replace it later.
    #
    # Note that these references may or may not exist. We expect the
    # go.mod to have a replace statement _if_  developer has subpackages
    # in their handler. In this case they will need a this replace statement
    #
    #    replace handler/function => ./
    #
    # `go mod` will then add a line that looks like
    #
    #    handler/function v0.0.0-00010101000000-000000000000
    #
    # both of these lines need to be replaced, this grep selects everything
    # _except_ those offending lines.
    grep -v "\shandler/function" go.mod > gomod2; mv gomod2 go.mod

    # Now update the go.mod
    #
    # 1. use replace so that imports of handler/function use the local code
    # 2. we need to rename the module to handler because our main.go assumes
    #    this is the package name
    go mod edit \
        -replace=handler/function=./function \
        -module handler



    if [ "$DEBUG" -eq 1 ]; then
        cat go.mod
        echo ""
    fi
}


# cleanup_vendor_modulestxt will cleanup the modules.txt file in the vendor folder
# this file is needed when modules are enabled and it must be in sync with the
# go.mod.  To function correctly we need to modify the references to handler/function,
# if they exist.
cleanup_vendor_modulestxt() {
    if [ ! -d ./vendor ]; then
        echo "no vendor found, skipping modules.txt cleanup"
        return
    fi

    # Nothing to do when modules is explicitly off
    # the z prefix protects against any SH wonkiness
    # see https://stackoverflow.com/a/18264223
    if [ "z$GO111MODULE" = "zoff" ]; then
        echo "modules disabled, skipping modules.txt cleanup"
        return;
    fi

    echo "cleanup vendor/modules.txt"

    # just in case
    touch "./vendor/modules.txt"

    # when vendored, we need to do similar edits to the vendor/modules.txt
    # as we did to the go.mod

    # 1. we need to replace any possible copy of the handler code
    rm -rf vendor/handler && \

    # 2. in modules.txt, we remove existing references to the handler/function
    #    we reconstruct these in the last step
    grep -v "\shandler/function" ./vendor/modules.txt> modulestext; mv modulestext ./vendor/modules.txt

    # 3. Handle any other local replacements.
    # any replace that points to `./**` needs to be udpat    echo "cleanup local replace statements"
    sed -i 's/=> \.\//=> \.\/function\//' ./vendor/modules.txt

    # 4. To make the modules.txt consistent with the new go.mod,
    #    we add the mising replace to the vendor/modules.txt
    echo "## explicit" >> ./vendor/modules.txt
    echo "# handler/function => ./function" >> ./vendor/modules.txt

    if [ "$DEBUG" -eq 1 ]; then
        cat ./vendor/modules.txt;
        echo ""
    fi
}

# has_local_replacement checks if the file contains local go module replacement
has_local_replacement() {
    return "$(grep -E -c '=> \./\S+' "$1")"
}


################
#   main
################
move_vendor

cleanup_gomod

cleanup_vendor_modulestxt
