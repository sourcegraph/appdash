# AppDash: Python

## Setup

To install appdash for python you'll first need to install Google's protobuf and the Twisted networking library through the standard `easy_install` python command:

```
# (Ubuntu/Linux) Install easy_install:
sudo apt-get install python-setuptools

# Install Google's protobuf:
easy_install protobuf

# Install Twisted networking:
easy_install twisted
```

Depending on where Python is installed and/or what permissions the directory has, you may need to run the above `easy_install` commands as root.

## Testing

If all is well with your setup, you should be able to change directory to here and run the example:

```
# Run appdash server in separate terminal:
appdash serve

# Run the example script:
cd $GOPATH/src/sourcegraph.com/sourcegraph/appdash/python
./example.py
```

## Integration

TODO(slimsag): integrate nicely with common Python HTTP frameworks.
TODO(slimsag): provide way to easily install python' appdash module.

