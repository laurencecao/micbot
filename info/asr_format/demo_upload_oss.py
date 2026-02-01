import oss2
from datetime import datetime, timedelta

# 1. 阿里云账号AccessKey拥有所有API的访问权限，风险很高。
# 强烈建议您创建并使用RAM用户进行API访问或日常运维，请登录RAM控制台创建RAM用户。
access_key_id = ''
access_key_secret = ''

# 2. 填写Bucket所在地域的Endpoint。以华东1（杭州）为例，Endpoint填写为https://oss-cn-hangzhou.aliyuncs.com。
endpoint = 'https://oss-cn-shanghai.aliyuncs.com'

# 3. 填写Bucket名称。
bucket_name = 'oss-pai-rmia4s0uublyu39v4o-cn-shanghai'

# 4. 填写Bucket名称。
auth = oss2.Auth(access_key_id, access_key_secret)
bucket = oss2.Bucket(auth, endpoint, bucket_name)

# 5. 上传本地文件
# 'exampleobject.txt' 是上传到OSS后的文件名，'localfile.txt' 是本地文件路径
# 如果没有指定本地文件路径，则默认上传到当前目录
object_name = "resp_funasr.json"
bucket.put_object_from_file(object_name, object_name)
print("文件上传成功")

now = datetime.now()
print(f"Current time: {now}")
hours_to_add = 1
time_delta = timedelta(hours=hours_to_add)
expiration_time = now + time_delta
try:
    https_url = bucket.sign_url('GET', object_name, expires=10)
    print(f"Generated HTTPS URL: {https_url}")
except Exception as e:
    print(f"An error occurred: {e} ")

