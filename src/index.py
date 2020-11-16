# -*- coding: utf-8 -*-

import os
import oss2
import json
from PIL import Image, ImageFont, ImageDraw

sourceBucket = None
targetBucket = None


def initializer(context):
    endpoint = 'https://oss-%s-internal.aliyuncs.com' % context.region
    global sourceBucket
    global targetBucket
    sourceBucket = get_oss_client(context, endpoint, os.environ.get('SOURCE_BUCKET'))
    targetBucket = get_oss_client(context, endpoint, os.environ.get('TARGET_BUCKET'))


# 图片水印
def watermarImage(image, watermarkStr):
    font = ImageFont.truetype("Brimborion.ttf", 40)
    drawImage = ImageDraw.Draw(image)
    height = []
    width = []
    for eveStr in watermarkStr:
        thisWidth, thisHeight = drawImage.textsize(eveStr, font)
        height.append(thisHeight)
        width.append(thisWidth)
    drawImage.text((image.size[0] - sum(width) - 10, image.size[1] - max(height) - 10),
                   watermarkStr, font=font,
                   fill=(255, 255, 255, 255))

    return image


# 图片压缩
def compressImage(image, width):
    height = image.size[1] / (image.size[0] / width)
    return image.resize((int(width), int(height)))


def handler(event, context):

    event = json.loads(event.decode("utf-8"))

    for eveEvent in event["events"]:
        # 获取object
        print("获取object")
        image = eveEvent["oss"]["object"]["key"]
        localFileName = "/tmp/" + event["events"][0]["oss"]["object"]["eTag"]
        localReadyName = localFileName + "-result.png"

        # 下载图片
        print("下载图片")
        print("image: ", image)
        print("localFileName: ", localFileName)
        sourceBucket.get_object_to_file(image, localFileName)

        # 图像压缩
        print("图像压缩")
        imageObj = Image.open(localFileName)
        imageObj = compressImage(imageObj, width=500)
        imageObj = watermarImage(imageObj, "Hello Serverless Devs")
        imageObj.save(localReadyName)

        # 数据回传
        print("数据回传")
        with open(localReadyName, 'rb') as fileobj:
            targetBucket.put_object(image, fileobj.read())
        print("Url: ", "http://" + os.environ.get('TARGET_BUCKET') + "/" + image)

    return 'oss trigger'


def get_oss_client(context, endpoint, bucket):
    creds = context.credentials
    if creds.security_token is not None:
        auth = oss2.StsAuth(creds.access_key_id, creds.access_key_secret, creds.security_token)
    else:
        # for local testing, use the public endpoint
        endpoint = str.replace(endpoint, "-internal", "")
        auth = oss2.Auth(creds.access_key_id, creds.access_key_secret)
    return oss2.Bucket(auth, endpoint, bucket)