make build_server
echo 'uploading'
rsync -a ./bin/reverse_server root@silviastom.ru:/root/reverse/
echo 'ssh-ing'
ssh root@silviastom.ru 'systemctl restart reverse'