"""
定时任务管理
"""

from datetime import datetime

from apscheduler.schedulers.background import BackgroundScheduler
from apscheduler.triggers.cron import CronTrigger
from apscheduler.triggers.interval import IntervalTrigger


class TaskScheduler:
    """量化交易定时任务调度器"""

    def __init__(self):
        self.scheduler = BackgroundScheduler(
            timezone="Asia/Shanghai",
            job_defaults={
                "coalesce": True,
                "max_instances": 1,
                "misfire_grace_time": 300,
            },
        )
        self._jobs = {}

    def start(self):
        """启动调度器"""
        if not self.scheduler.running:
            self.scheduler.start()
            print(f"[Scheduler] Started at {datetime.now()}")

    def stop(self):
        """停止调度器"""
        if self.scheduler.running:
            self.scheduler.shutdown(wait=False)
            print(f"[Scheduler] Stopped at {datetime.now()}")

    def schedule_daily_prediction(self, func, hour=15, minute=30):
        """
        每日收盘后执行预测（默认 15:30）
        :param func: 预测函数
        """
        job_id = "daily_prediction"
        trigger = CronTrigger(hour=hour, minute=minute, timezone="Asia/Shanghai")
        job = self.scheduler.add_job(
            func,
            trigger=trigger,
            id=job_id,
            name="每日预测",
            replace_existing=True,
        )
        self._jobs[job_id] = job
        print(f"[Scheduler] Scheduled daily prediction at {hour}:{minute}")
        return job_id

    def schedule_weekly_training(self, func, day_of_week="sat", hour=2, minute=0):
        """
        每周重训练模型（默认周六凌晨 2:00）
        :param func: 训练函数
        """
        job_id = "weekly_training"
        trigger = CronTrigger(
            day_of_week=day_of_week,
            hour=hour,
            minute=minute,
            timezone="Asia/Shanghai",
        )
        job = self.scheduler.add_job(
            func,
            trigger=trigger,
            id=job_id,
            name="每周重训练",
            replace_existing=True,
        )
        self._jobs[job_id] = job
        print(f"[Scheduler] Scheduled weekly training on {day_of_week} at {hour}:{minute}")
        return job_id

    def schedule_model_evaluation(self, func, hour=16, minute=0):
        """
        每日评估模型准确率（默认 16:00）
        :param func: 评估函数
        """
        job_id = "model_evaluation"
        trigger = CronTrigger(hour=hour, minute=minute, timezone="Asia/Shanghai")
        job = self.scheduler.add_job(
            func,
            trigger=trigger,
            id=job_id,
            name="每日模型评估",
            replace_existing=True,
        )
        self._jobs[job_id] = job
        print(f"[Scheduler] Scheduled model evaluation at {hour}:{minute}")
        return job_id

    def schedule_interval(self, func, job_id, minutes=60):
        """
        间隔执行任务
        :param func: 任务函数
        :param job_id: 任务 ID
        :param minutes: 间隔分钟数
        """
        trigger = IntervalTrigger(minutes=minutes, timezone="Asia/Shanghai")
        job = self.scheduler.add_job(
            func,
            trigger=trigger,
            id=job_id,
            name=job_id,
            replace_existing=True,
        )
        self._jobs[job_id] = job
        print(f"[Scheduler] Scheduled {job_id} every {minutes} minutes")
        return job_id

    def remove_job(self, job_id):
        """移除任务"""
        if job_id in self._jobs:
            self.scheduler.remove_job(job_id)
            del self._jobs[job_id]
            print(f"[Scheduler] Removed job: {job_id}")

    def list_jobs(self):
        """列出所有已注册任务"""
        jobs = []
        for job in self.scheduler.get_jobs():
            jobs.append({
                "id": job.id,
                "name": job.name,
                "next_run": str(job.next_run_time) if job.next_run_time else None,
                "trigger": str(job.trigger),
            })
        return jobs

    def get_status(self):
        """获取调度器状态"""
        return {
            "running": self.scheduler.running,
            "job_count": len(self.scheduler.get_jobs()),
            "jobs": self.list_jobs(),
        }


# ──────────────────────────────────────────────
# 自测入口
# ──────────────────────────────────────────────

if __name__ == "__main__":
    import time

    scheduler = TaskScheduler()

    def demo_prediction():
        print(f"[Demo] Running daily prediction at {datetime.now()}")

    def demo_training():
        print(f"[Demo] Running weekly training at {datetime.now()}")

    def demo_evaluation():
        print(f"[Demo] Running model evaluation at {datetime.now()}")

    # 注册任务
    scheduler.schedule_daily_prediction(demo_prediction, hour="*", minute="*")
    scheduler.schedule_weekly_training(demo_training, day_of_week="*", hour="*", minute="*")
    scheduler.schedule_model_evaluation(demo_evaluation, hour="*", minute="*")

    scheduler.start()
    print("Jobs:", scheduler.list_jobs())
    print("Status:", scheduler.get_status())

    # 让调度器运行一会
    time.sleep(3)
    scheduler.stop()
    print("Done.")