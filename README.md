# README.md
支持Windows和macOS双平台，用户可通过执行一个可可行文件开启go-copy-samba服务。
实现的feature包括：
1. 本地建立与CDS的映射关系并且采取一定的保密措施，限制白盒用户的权限操作。
2. 开启的Websocket服务支持本地文件至映射目录/盘符的拷贝操作。
3. 通过Golang原生的异步队列任务完成（多线程/协程）拷贝任务管理，通过用户本机电脑的CPU物理核数开启双倍的并发执行线程。
4. 加入了拷贝任务的超时管理，通过设置常量超时时间，任务超时后可自动退出。
5. 通过TimeTiker加入了对文件传输中的实时监控，重写io.Reader的Read方法实现了原生io.Copy过程中的字节数实时记录更新的操作。
6. 所有文件传输的生命周期(UUID,Source,Dest,Size,Status,Created,Updated...)存入Sqlite数据库。
7. 通过Logrus package 完成日志记录，所有记录均为官方推荐的JSON fields方式，针对每个功能的细节进行记录追踪，每个功能模块进行LogTag的标记，方便排查检错。通过Logrus强大的Hook功能，后续在发生错误时可以将错误发送邮件至管理维护者。