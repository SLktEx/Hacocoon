packer {
  required_version = "= 1.16.0"
  required_plugins {
    incus = {
      source  = "github.com/bketelsen/incus"
      version = "= 1.0.5"
    }
  }
}

variable "build_id" {
  type    = string
  default = env("HACO_PACKER_BUILD_ID")
}

variable "tool_message" {
  type    = string
  default = "hello-from-packer"
}

variable "image" {
  type    = string
  default = "images:ubuntu/26.04"
}

source "incus" "base" {
  image          = var.image
  output_image   = "haco-packer-${var.build_id}"
  container_name = "haco-packer-${var.build_id}"
  profile        = "default"

  launch_config = {
    "user.hacocoon.packer-build" = var.build_id
    "limits.cpu"                = "2"
    "limits.memory"             = "4GiB"
    "limits.processes"          = "1024"
  }

  publish_properties = {
    "user.hacocoon.packer-build" = var.build_id
  }
}

build {
  sources = ["source.incus.base"]

  provisioner "shell" {
    # The guest may still mount/clear /tmp after Incus reports it running.
    remote_folder    = "/root"
    script           = "setup.sh"
    environment_vars = ["TOOL_MESSAGE=${var.tool_message}"]
  }
}
