resource "local_file" "helloworld" {
  content  = "hello world!"
  filename = "hello.txt"
}

resource "null_resource" "example_dir" {
  provisioner "local-exec" {
    command = "mkdir -p ${path.module}/new_directory"
  }
}
