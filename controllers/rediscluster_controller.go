/*
Copyright 2020 Opstree Solutions.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"strconv"
	"time"

	"redis-operator/k8sutils"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	redisv1beta1 "redis-operator/api/v1beta1"
)

// RedisClusterReconciler reconciles a RedisCluster object
type RedisClusterReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// Reconcile is part of the main kubernetes reconciliation loop
func (r *RedisClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger := r.Log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	// 1.6611548692645001e+09  INFO    controllers.Redis	Reconciling opstree redis Cluster controller    {"Request.Namespace": "default", "Request.Name": "create"}
	reqLogger.Info("Reconciling opstree redis Cluster controller")
	instance := &redisv1beta1.RedisCluster{}

	// 从本地缓存中读取 这里的Get方法底层实际调用的是delegatingReader的Get方法，delegatingReader的CacheReader属性的实际类型是 informerCache，其Get方法就是informerCache.Get
	err := r.Client.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if _, found := instance.ObjectMeta.GetAnnotations()["rediscluster.opstreelabs.in/skip-reconcile"]; found {
		reqLogger.Info("Found annotations rediscluster.opstreelabs.in/skip-reconcile, so skipping reconcile")
		return ctrl.Result{RequeueAfter: time.Second * 10}, nil
	}

	// 获得副本数量的指针
	leaderReplicas := instance.Spec.GetReplicaCounts("leader")
	followerReplicas := instance.Spec.GetReplicaCounts("follower")
	totalReplicas := leaderReplicas + followerReplicas

	// 如果当前实例被标记了删除，那么删除该实例关联的service和pvc
	if err := k8sutils.HandleRedisClusterFinalizer(instance, r.Client); err != nil {
		return ctrl.Result{RequeueAfter: time.Second * 60}, err
	}

	// 如果没有的话，给实例添加一个 Finalizer
	if err := k8sutils.AddRedisClusterFinalizer(instance, r.Client); err != nil {
		return ctrl.Result{RequeueAfter: time.Second * 60}, err
	}

	// 创建所有的主节点
	err = k8sutils.CreateRedisLeader(instance)
	if err != nil {
		return ctrl.Result{RequeueAfter: time.Second * 60}, err
	}
	if leaderReplicas != 0 {
		err = k8sutils.CreateRedisLeaderService(instance)
		if err != nil {
			return ctrl.Result{RequeueAfter: time.Second * 60}, err
		}
	}

	err = k8sutils.ReconcileRedisPodDisruptionBudget(instance, "leader", instance.Spec.RedisLeader.PodDisruptionBudget)
	if err != nil {
		return ctrl.Result{RequeueAfter: time.Second * 60}, err
	}

	redisLeaderInfo, err := k8sutils.GetStatefulSet(instance.Namespace, instance.ObjectMeta.Name+"-leader")
	if err != nil {
		return ctrl.Result{RequeueAfter: time.Second * 60}, err
	}

	if int32(redisLeaderInfo.Status.ReadyReplicas) == leaderReplicas {
		err = k8sutils.CreateRedisFollower(instance)
		if err != nil {
			return ctrl.Result{RequeueAfter: time.Second * 60}, err
		}
		// if we have followers create their service.
		if followerReplicas != 0 {
			err = k8sutils.CreateRedisFollowerService(instance)
			if err != nil {
				return ctrl.Result{RequeueAfter: time.Second * 60}, err
			}
		}
		err = k8sutils.ReconcileRedisPodDisruptionBudget(instance, "follower", instance.Spec.RedisFollower.PodDisruptionBudget)
		if err != nil {
			return ctrl.Result{RequeueAfter: time.Second * 60}, err
		}
	}
	redisFollowerInfo, err := k8sutils.GetStatefulSet(instance.Namespace, instance.ObjectMeta.Name+"-follower")
	if err != nil {
		return ctrl.Result{RequeueAfter: time.Second * 60}, err
	}

	if leaderReplicas == 0 {
		reqLogger.Info("Redis leaders Cannot be 0", "Ready.Replicas", strconv.Itoa(int(redisLeaderInfo.Status.ReadyReplicas)), "Expected.Replicas", leaderReplicas)
		return ctrl.Result{RequeueAfter: time.Second * 120}, nil
	}

	if int32(redisLeaderInfo.Status.ReadyReplicas) != leaderReplicas && int32(redisFollowerInfo.Status.ReadyReplicas) != followerReplicas {
		reqLogger.Info("Redis leader and follower nodes are not ready yet", "Ready.Replicas", strconv.Itoa(int(redisLeaderInfo.Status.ReadyReplicas)), "Expected.Replicas", leaderReplicas)
		return ctrl.Result{RequeueAfter: time.Second * 120}, nil
	}

	// 前面已经确保了pod数量是够的，剩下的就是实际的redis cluster集群的节点数量
	reqLogger.Info("Creating redis cluster by executing cluster creation commands", "Leaders.Ready", strconv.Itoa(int(redisLeaderInfo.Status.ReadyReplicas)), "Followers.Ready", strconv.Itoa(int(redisFollowerInfo.Status.ReadyReplicas)))
	if k8sutils.CheckRedisNodeCount(instance, "") != totalReplicas {
		// todo 这个地方通过cluster nodes获取节点数量，但只检查节点数量，不检查节点状态，失败的节点也算
		leaderCount := k8sutils.CheckRedisNodeCount(instance, "leader")
		if leaderCount != leaderReplicas {
			// 主节点数量不够，有可能是第一次集群创建，或者是有redis中的node莫名的离开了集群（只能手动断开，因为网络分区等不会导致cluster nodes的输出减少）
			// 接着排除手动断开的话，这个分支99%的可能是集群第一次创建的时候
			reqLogger.Info("Not all leader are part of the cluster...", "Leaders.Count", leaderCount, "Instance.Size", leaderReplicas)
			k8sutils.ExecuteRedisClusterCommand(instance)
		} else {
			if followerReplicas > 0 {
				reqLogger.Info("All leader are part of the cluster, adding follower/replicas", "Leaders.Count", leaderCount, "Instance.Size", leaderReplicas, "Follower.Replicas", followerReplicas)
				k8sutils.ExecuteRedisReplicationCommand(instance)
			} else {
				reqLogger.Info("no follower/replicas configured, skipping replication configuration", "Leaders.Count", leaderCount, "Leader.Size", leaderReplicas, "Follower.Replicas", followerReplicas)
			}
		}
	} else {
		reqLogger.Info("Redis leader count is desired")
		// 检查是否有flag是fail或者连接状态是disconnected
		if k8sutils.CheckRedisClusterState(instance) >= int(totalReplicas)-1 {
			reqLogger.Info("Redis leader is not desired, executing failover operation")
			//  这个地方判断至少是整个集群中所有节点状态都不对的情况下，才把集群铲掉重建
			err = k8sutils.ExecuteFailoverOperation(instance)
			if err != nil {
				return ctrl.Result{RequeueAfter: time.Second * 10}, err
			}
		}
		// 否则的话，等待集群调谐 2分钟一次
		return ctrl.Result{RequeueAfter: time.Second * 120}, nil
	}
	reqLogger.Info("Will reconcile redis cluster operator in again 10 seconds")
	return ctrl.Result{RequeueAfter: time.Second * 10}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RedisClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&redisv1beta1.RedisCluster{}).
		Complete(r)
}
