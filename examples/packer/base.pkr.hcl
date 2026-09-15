variable "tool_message" {
  type    = string
  default = "hello-from-packer"
}

variable "haco_packer_port" {
  type    = string
  default = env("HACO_PACKER_PORT")
}

variable "haco_packer_key" {
  type    = string
  default = env("HACO_PACKER_KEY")
}

source "null" "base" {
  ssh_host                     = "127.0.0.1"
  ssh_port                     = tonumber(var.haco_packer_port)
  ssh_username                 = "root"
  ssh_private_key_file         = var.haco_packer_key
  ssh_agent_auth               = false
  ssh_disable_agent_forwarding = true
}

build {
  sources = ["source.null.base"]
  provisioner "shell" {
    script           = "setup.sh"
    environment_vars = ["TOOL_MESSAGE=${var.tool_message}"]
  }
}
