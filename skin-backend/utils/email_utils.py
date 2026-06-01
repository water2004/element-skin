import logging
import aiosmtplib
from email.mime.text import MIMEText
from email.mime.multipart import MIMEMultipart
from email.header import Header
from email.utils import formataddr, parseaddr
from database_module import Database

logger = logging.getLogger(__name__)

class EmailSender:
    def __init__(self, db: Database):
        self.db = db

    async def _get_settings(self):
        settings = await self.db.setting.get_all()
        return {
            "host": settings.get("smtp_host", ""),
            "port": int(settings.get("smtp_port", "465")),
            "user": settings.get("smtp_user", ""),
            "password": settings.get("smtp_password", ""),
            "ssl": settings.get("smtp_ssl", "true") == "true",
            "sender": settings.get("smtp_sender", ""),
            "enabled": settings.get("email_verify_enabled", "false") == "true"
        }

    async def send_verification_code(self, to_email: str, code: str, type: str):
        settings = await self._get_settings()
        if not settings["enabled"]:
            return False

        if not settings["host"]:
             logger.warning("SMTP host not configured.")
             return False

        subject = "SkinServer 验证码"
        if type == "register":
            body = f"""
            <html>
            <body>
                <h2>欢迎注册 SkinServer</h2>
                <p>您的验证码是：<strong style="font-size: 20px; color: #409EFF;">{code}</strong></p>
                <p>该验证码将在几分钟后过期，请尽快完成注册。</p>
            </body>
            </html>
            """
        elif type == "reset":
            body = f"""
            <html>
            <body>
                <h2>SkinServer 密码重置</h2>
                <p>您正在进行密码重置操作。</p>
                <p>您的验证码是：<strong style="font-size: 20px; color: #409EFF;">{code}</strong></p>
                <p>如果这不是您本人的操作，请忽略此邮件。</p>
            </body>
            </html>
            """
        else:
            return False

        message = MIMEMultipart()
        
        # RFC-compliant From header construction
        sender_name, sender_addr = parseaddr(settings["sender"])
        # Fallback to smtp_user if sender address is empty
        if not sender_addr and settings["user"]:
            sender_addr = settings["user"]
        
        if sender_name:
            message["From"] = formataddr((Header(sender_name, 'utf-8').encode(), sender_addr))
        else:
            message["From"] = sender_addr

        message["To"] = to_email
        message["Subject"] = Header(subject, 'utf-8')
        message.attach(MIMEText(body, "html", "utf-8"))

        try:
            await aiosmtplib.send(
                message,
                hostname=settings["host"],
                port=settings["port"],
                username=settings["user"],
                password=settings["password"],
                use_tls=settings["ssl"],
            )
            return True
        except Exception as e:
            logger.error("Failed to send email: %s", e)
            return False
