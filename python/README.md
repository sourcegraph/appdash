# AppDash: Python

## Setup

To install appdash for python you'll first need to install a few things through the standard `easy_install` and `pip` python package managers:

```
# (Ubuntu/Linux) Install easy_install and pip:
sudo apt-get install python-setuptools python-pip

# Install Google's protobuf:
easy_install protobuf

# Install Twisted networking (optional, only needed for Twisted integration):
easy_install twisted

# Install strict-rfc3339
pip install strict-rfc3339
```

Depending on where Python is installed and/or what permissions the directory has, you may need to run the above `easy_install` and `pip` commands as root.

## Twisted Example

If all is well with your setup, you should be able to change directory to `sourcegraph.com/sourcewgraph/appdash/python` and run the Twisted example:

```
# Run appdash server in separate terminal:
appdash serve

# Run the example script:
cd $GOPATH/src/sourcegraph.com/sourcegraph/appdash/python
./example_twisted.py
```

## TODO

- Integrate nicely with common Python HTTP frameworks.
- Provide a way to easily install python' appdash module.

