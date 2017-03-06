# rkt stage1-skim

A [rkt](https://github.com/coreos/rkt) flavor for running pods natively on a host.  In case you need full access
to the host, but don't want the application or pod cluttering up your filesystem.

## Dependencies and Building

You will need Go (at least 1.6 for the versioning), and [Glide](https://github.com/Masterminds/glide)

To build the stage1 flavor:

```
./build.sh
```

For your benefit, `build.sh` also accepts a single argument `clean` that will
remove all build artifacts from the build tree.

## Using

Skim is based off of [fly](https://coreos.com/rkt/docs/latest/running-fly-stage1.html)
which is an another stage1 flavor for rkt designed to run your application pod inside of
a chroot environment.  Just like fly, skim can only run one executable at a time.  In
addition, the executable defined in your ACI pod image, or passed via `rkt run`, will
need to be specified as an absolute path.  This is due to the path mangling skim does
to ensure the correct binary is invoked.

Otherwise, place `stage1-skim.aci` into your rkt stage1 path or specify the handler
using something along the lines of `--stage1-path`.

Lastly, you will need to pass the following into rkt as the stage1 flavor is not signed:

    --insecure-options=image

## Miscellaneous

This code is copyright under the Apache-2 license
