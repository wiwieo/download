# download
分块下载，多并发，提高下载速度

# 流程简述
1、启动后，等待下载任务到来

2、将接收到的下载任务，分割成指定个数的子任务，使用协程同时下载

3、将子任务下载的内容写入文件

4、所有子任务下载完成后合并成一个文件

5、通知主程，下载完毕
