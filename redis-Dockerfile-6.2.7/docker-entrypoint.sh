#!/bin/sh

# -e 若指令传回值不等于0，则立即退出shell
set -ea

# first arg is `-f` or `--some-option` or first arg is `something.conf`
# ${1#-} 表示如果变量以-开头，就从$1变量中删除-， "${1#-}" != "$1" 的意思就是如果$1参数是以-开头的，那么条件成立
# ${1%.conf}操作表示如果$1参数是以.conf结尾的，就删除结尾的.conf. "${1%.conf}" != "$1" 判断$1参数知否携带.conf后缀，如果携带了，则条件成立
if [ "${1#-}" != "$1" ] || [ "${1%.conf}" != "$1" ]; then
  # Linux set命令用于设置shell， 许多命令的输出是以空格分隔的值，如果要使用其中的某个数据域，使用 set 非常有效。
  # 比如这里如果第一个参数是 /etc/redis/redis.conf， 则set之前$@为/etc/redis/redis.conf，而经过 set -- redis-server "$@" 后，$@就变成了  redis-server /etc/redis/redis.conf
  # $@ 表示外部传入的所有位置参数，如果有多个，就是类似 {$1, $2, $3 ...} 这样的类数组格式
  # 比如外部参数如果是 "/etc/redis/redis.conf"，则这里就是 set -- redis-server "/etc/redis/redis.conf"
  # 该指令执行完以后，echo $@将输出 redis-server /etc/redis/redis.conf
  # 如果是sentinel模式运行，echo $@将输出 redis-server /etc/redis/redis.conf --sentinel
  set -- redis-server "$@"
fi

# allow the container to be started with `--user`
# 第一次root用户进程进来时，$1参数就是CMD里面的redis-server， 并且用户是root
# 第二次redis用户进程进来时，由于用户id=999，不是root了，所以不会执行下面的内容
if [ "$1" = 'redis-server' -a "$(id -u)" = '0' ]; then
  # -user redis 所有者为redis的文件
  # chown redis 将指定文件的拥有者改为指redis
  # 将当前目录所有拥有者不是redis的文件的所有者改为redis
  # {}是找到的所有文件的集合
  # +应该是递归
  # . 即当前目录，注意不是脚本所在的目录，而是当前工作目录，即/data
  find . \! -user redis -exec chown redis '{}' +
  # `$0`表示当前脚本的名称,即`docker-entrypoint.sh`
  # `$@`表示外部传入的所有位置参数，如果有多个，就是类似 {$1, $2, $3 ...} 这样的类数组格式，这里即 `redis-server`
  # `gosu redis "$0" "@"` 前面加上个exec，表示以`gosu redis "$0" "@"`这个命令启动的进程(exec gosu redis启动了一个新的进程)替换正在执行的docker-entrypoint.sh进程(即root用户执行docker-entrypoint.sh脚本的进程)，这样就保证了`gosu redis "$0" "@"`对应的进程ID为1
  #  exec gosu redis "$0" "$@" 中的redis指的是redis用户，即这里使用redis用户再运行一次docker-entrypoint.sh脚本
  #  所以最终的指令可能是 gosu redis docker-entrypoint.sh "redis-server /etc/redis/redis.conf"
  exec gosu redis "$0" "$@"
fi

# set an appropriate umask (if one isn't set already)
# - https://github.com/docker-library/redis/issues/305
# - https://github.com/redis/redis/blob/bb875603fb7ff3f9d19aad906bd45d7db98d9a39/utils/systemd-redis_server.service#L37
um="$(umask)"
if [ "$um" = '0022' ]; then
  umask 0077
fi

EXTERNAL_CONFIG_FILE=${EXTERNAL_CONFIG_FILE:-"/etc/redis/external.conf.d/redis-external.conf"}
DATA_DIR=${DATA_DIR:-"/data"}

#输出一下指定的用户信息
current_user="$(id)"
echo "exec user is [${current_user}]"
echo "shell args list " "$@"

# 添加扩展配置文件，扩展配置文件内容不可被config rewrite重写，更准确的说，是CONFIG REWRITE只会将配置重写到redis.conf文件中
# redis总是采用最后一行的配置作为最终的配置，所以当扩展的配置文件放在redis.conf头部时，redis.conf中的配置不会被扩展的配置文件覆盖
external_config() {
  echo "include ${EXTERNAL_CONFIG_FILE}" >>"${DATA_DIR}/redis.conf"
  # 这个配置添加到文件第一行，防止覆盖默认配置 sed -i "1i\include ${EXTERNAL_CONFIG_FILE}" "${DATA_DIR}/redis.conf"
}

write_cluster_config(){
  {
    echo cluster-enabled yes
    echo cluster-node-timeout 15000
    echo cluster-require-full-coverage no
    echo cluster-migration-barrier 1
    echo cluster-config-file "${DATA_DIR}/nodes.conf"
    echo aclfile "${DATA_DIR}/users.acl"
    echo acl-pubsub-default resetchannels
  } >>"${DATA_DIR}/redis.conf"
}

write_pod_ip() {
  if [ -f "${DATA_DIR}/nodes.conf" ]; then
    # 如果没有指定POD_IP环境变量，则使用 hostname -i
    if [ -z "${POD_IP}" ]; then
      POD_IP=$(hostname -i)
    fi
    echo "${DATA_DIR}/nodes.conf" "is exists, update myself pod ip to ${POD_IP}"
    sed -i -e "/myself/ s/[0-9]\{1,3\}\.[0-9]\{1,3\}\.[0-9]\{1,3\}\.[0-9]\{1,3\}/${POD_IP}/" "${DATA_DIR}/nodes.conf"
  fi
}

create_users_acl(){
  touch "${DATA_DIR}/users.acl"
}

# 重要提示：从Redis 6开始，"requirepass "只是新ACL系统之上的一个兼容层。该选项的作用只是为default用户设置密码。
# 客户端仍将像往常一样使用AUTH <password>进行认证，或者更明确地使用AUTH default <password>，如果他们遵循新的协议：两者都可以工作。requirepass 与 aclfile 选项和 ACL LOAD 命令不兼容，这些将导致 requirepass 被忽略
set_redis_password() {
    if [ -z "${REDIS_PASSWORD}" ]; then
        echo "Redis is running without password which is not recommended"
    else
      sed -i '/masterauth/d' "${DATA_DIR}/redis.conf"
      sed -i '/requirepass/d' "${DATA_DIR}/redis.conf"
      {
          echo masterauth "${REDIS_PASSWORD}"
          echo requirepass "${REDIS_PASSWORD}"
      } >>"${DATA_DIR}/redis.conf"
    fi
}

# 写集群配置
redis_mode_setup() {
  # cluster模式启动
  if [ "${SETUP_MODE}" = "cluster" ]; then
    # 如果redis.conf配置文件不存在，则创建初始化配置
    if [ ! -e "${DATA_DIR}/redis.conf" ]; then
      echo "${DATA_DIR}/redis.conf" "is not exists, current cluster first startup, create ${DATA_DIR}/redis.conf"

      # 1、写入扩展配置
      if [ -f "${EXTERNAL_CONFIG_FILE}" ]; then
        external_config
      fi

      # 2、写入集群配置
      write_cluster_config

      # 3、创建acl文件
      create_users_acl
    fi

    # 5、设置default用户密码
    set_redis_password

    # 4、写入最新ip到node.conf文件
    write_pod_ip
  else
    echo "Setting up redis in standalone mode"
  fi
}

# 从此处开始，就是redis用户在执行了
redis_mode_setup


# `$@`前面有个exec，会用`redis-server`命令启动的进程取代当前的`docker-entrypoint.sh`进程，所以，最终redis进程的PID等于1，`而docker-entrypoint.sh`这个脚本的进程已经被替代，因此就结束掉了；
# exec redis-server /etc/redis/redis.conf
exec "$@"
