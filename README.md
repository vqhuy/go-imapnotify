# go-imapnotify
`imapnotify` reimplemented in Go

### Config
```
Host = "imap.gmail.com"
Port = 993
Username = "my_email_address@gmail.com"
Password = "******"
# PasswdCmd = "echo mypassword.txt"
OnNotify = "mbsync -a"
OnNotifyPost = "emacsclient  -e '(mu4e-update-index)'"
Boxes = ["*"]
```

See
https://martinralbrecht.wordpress.com/2016/05/30/handling-email-with-emacs/
for an application of this piece.