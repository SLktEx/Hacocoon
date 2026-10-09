"""Private, descriptor-relative receipts for the standalone Incus VM probe."""
import json
import os
import re
import stat
import tempfile
import uuid

NAME = re.compile(r"hci-\d{1,20}-\d{1,10}-vm\Z", re.ASCII)
RECEIPT = "receipt.json"
LIMIT = 64 * 1024
DIRECTORY_FLAGS = os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC
FILE_FLAGS = os.O_NOFOLLOW | os.O_CLOEXEC


class ProbeError(Exception):
    pass


class NotReserved(Exception):
    """The private run directory does not exist; no reservation was made."""


class ReceiptStore:
    def __init__(self, name, create=False):
        if not isinstance(name, str) or not NAME.fullmatch(name):
            raise ProbeError("invalid_name")
        self.name = name
        self.fd = -1
        self.identity = None
        # CLI arguments cannot select a directory or filename. The standard
        # process temp root is pinned before selecting this bounded child name.
        self.directory_name = f"hacocoon-incus-vm-probe-{os.geteuid()}-{name}"
        root = os.open(tempfile.gettempdir(), DIRECTORY_FLAGS)
        try:
            if create:
                os.mkdir(self.directory_name, 0o700, dir_fd=root)
                os.fsync(root)
            self.fd = self._open_directory(root, create)
            if create:
                self._reserve()
            self.identity = self._checked_file_identity()
        except BaseException:
            self.close()
            raise
        finally:
            os.close(root)

    def _open_directory(self, root, create):
        try:
            fd = os.open(self.directory_name, DIRECTORY_FLAGS, dir_fd=root)
        except FileNotFoundError as exc:
            if not create:
                raise NotReserved() from exc
            raise
        info = os.fstat(fd)
        if info.st_uid != os.geteuid() or stat.S_IMODE(info.st_mode) != 0o700:
            os.close(fd)
            raise ProbeError("unsafe_receipt_directory")
        return fd

    def _reserve(self):
        fd = os.open(RECEIPT, os.O_WRONLY | os.O_CREAT | os.O_EXCL | FILE_FLAGS, 0o600, dir_fd=self.fd)
        with os.fdopen(fd, "w") as target:
            target.write("{}\n")
            target.flush()
            os.fsync(target.fileno())
        os.fsync(self.fd)

    @staticmethod
    def _file_identity(info):
        if (not stat.S_ISREG(info.st_mode) or info.st_uid != os.geteuid()
                or stat.S_IMODE(info.st_mode) != 0o600 or info.st_nlink != 1):
            raise ProbeError("unsafe_receipt_file")
        return info.st_dev, info.st_ino

    def _checked_file_identity(self):
        return self._file_identity(os.stat(RECEIPT, dir_fd=self.fd, follow_symlinks=False))

    def check_identity(self):
        if self._checked_file_identity() != self.identity:
            raise ProbeError("receipt_replaced")

    def read(self):
        self.check_identity()
        fd = os.open(RECEIPT, os.O_RDONLY | os.O_NONBLOCK | FILE_FLAGS, dir_fd=self.fd)
        with os.fdopen(fd, "rb") as source:
            info = os.fstat(source.fileno())
            if self._file_identity(info) != self.identity:
                raise ProbeError("receipt_replaced")
            raw = source.read(LIMIT + 1)
        if len(raw) > LIMIT:
            raise ProbeError("receipt_too_large")
        return json.loads(raw)

    def save(self, receipt):
        self.check_identity()
        temporary = ".receipt-" + uuid.uuid4().hex + ".tmp"
        fd = os.open(temporary, os.O_WRONLY | os.O_CREAT | os.O_EXCL | FILE_FLAGS, 0o600, dir_fd=self.fd)
        try:
            with os.fdopen(fd, "w") as target:
                target.write(json.dumps(receipt, indent=2) + "\n")
                target.flush()
                os.fsync(target.fileno())
                info = os.fstat(target.fileno())
            self.check_identity()
            os.replace(temporary, RECEIPT, src_dir_fd=self.fd, dst_dir_fd=self.fd)
            self.identity = info.st_dev, info.st_ino
            os.fsync(self.fd)
        finally:
            try:
                os.unlink(temporary, dir_fd=self.fd)
            except FileNotFoundError:
                pass

    def close(self):
        if self.fd >= 0:
            os.close(self.fd)
            self.fd = -1

    def __enter__(self):
        return self

    def __exit__(self, *_):
        self.close()
