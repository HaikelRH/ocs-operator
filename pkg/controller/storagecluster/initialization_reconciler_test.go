package storagecluster

import (
	"testing"

	nbv1 "github.com/noobaa/noobaa-operator/v2/pkg/apis/noobaa/v1alpha1"
	openshiftv1 "github.com/openshift/api/template/v1"
	api "github.com/openshift/ocs-operator/pkg/apis/ocs/v1"
	cephv1 "github.com/rook/rook/pkg/apis/ceph.rook.io/v1"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var logt = logf.Log.WithName("controller_storageclusterinitialization_test")

func TestRecreatingStorageClusterInitialization(t *testing.T) {
	cr := &api.StorageClusterInitialization{}
	request := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "ocsinit",
			Namespace: "",
		},
	}
	reconciler := createFakeInitializationStorageClusterReconciler(t, cr)
	result, err := reconciler.Reconcile(request)
	assert.NoError(t, err)
	assert.Equal(t, reconcile.Result{}, result)
}

func TestStorageClusterInitializationWithUnExpectedNamespace(t *testing.T) {
	cr := &api.StorageClusterInitialization{}
	request := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "ocsinit-test-not-found",
			Namespace: "ocsinit-test-not-found",
		},
	}
	reconciler := createFakeInitializationStorageClusterReconciler(t, cr)
	result, err := reconciler.Reconcile(request)
	assert.NoError(t, err)
	assert.Equal(t, reconcile.Result{}, result)
}

func TestInitStorageClusterWithOutResources(t *testing.T) {
	cr := &api.StorageClusterInitialization{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ocsinit",
		},
	}
	request := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "ocsinit",
			Namespace: "",
		},
	}
	reconciler := createFakeInitializationStorageClusterReconciler(t, cr)
	result, err := reconciler.Reconcile(request)
	assert.Equal(t, nil, err)
	assert.Equal(t, reconcile.Result{}, result)
}

func initStorageClusterResourceCreateUpdateTestWithPlatform(
	t *testing.T, platform *CloudPlatform, runtimeObjs []runtime.Object) {
	cr := createDefaultStorageCluster()
	request := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "ocsinit",
			Namespace: "",
		},
	}

	rtObjsToCreateReconciler := []runtime.Object{&nbv1.NooBaa{}}
	// runtimeObjs are present, it means tests are for update
	// add all the update required changes
	if runtimeObjs != nil {
		tbd := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "rook-ceph-tools",
			},
		}
		rtObjsToCreateReconciler = append(rtObjsToCreateReconciler, tbd)
	}

	reconciler := createFakeInitializationStorageClusterReconcilerWithPlatform(
		t, platform, rtObjsToCreateReconciler...)

	_ = reconciler.client.Create(nil, cr)
	for _, rtObj := range runtimeObjs {
		_ = reconciler.client.Create(nil, rtObj)
	}

	result, err := reconciler.Reconcile(request)
	assert.NoError(t, err)
	assert.Equal(t, reconcile.Result{}, result)

	assertExpectedResources(t, reconciler, cr, request)
}

func TestInitStorageClusterResourcesCreationOnAllPlatforms(t *testing.T) {
	allPlatforms := append(ValidCloudPlatforms,
		PlatformUnknown, CloudPlatformType("NonCloudPlatform"))
	for _, eachPlatform := range allPlatforms {
		initStorageClusterResourceCreateUpdateTestWithPlatform(
			t, &CloudPlatform{platform: eachPlatform}, nil)
	}
}

func createDefaultStorageCluster() *api.StorageCluster {
	return createStorageCluster("ocsinit", "zone", []string{"zone1", "zone2", "zone3"})
}

func createStorageCluster(scName, failureDomainName string,
	zoneTopologyLabels []string) *api.StorageCluster {
	cr := &api.StorageCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: scName,
		},
		Status: api.StorageClusterStatus{
			FailureDomain: failureDomainName,
			NodeTopologies: &api.NodeTopologyMap{
				Labels: map[string]api.TopologyLabelValues{
					zoneTopologyLabel: zoneTopologyLabels,
				},
			},
		},
	}
	return cr
}

func createUpdateRuntimeObjects(cp *CloudPlatform) []runtime.Object {
	csfs := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ocsinit-cephfs",
		},
	}
	csrbd := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ocsinit-ceph-rbd",
		},
	}
	cfs := &cephv1.CephFilesystem{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ocsinit-cephfilesystem",
		},
	}
	cbp := &cephv1.CephBlockPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ocsinit-cephblockpool",
		},
	}
	updateRTObjects := []runtime.Object{csfs, csrbd, cfs, cbp}

	// Create 'cephobjectstoreuser' only for non-aws platforms
	if cp.platform != PlatformAWS {
		cosu := &cephv1.CephObjectStoreUser{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ocsinit-cephobjectstoreuser",
			},
		}
		updateRTObjects = append(updateRTObjects, cosu)
	}

	// Create 'cephobjectstore' only for non-cloud platforms
	if !isValidCloudPlatform(cp.platform) {
		cos := &cephv1.CephObjectStore{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ocsinit-cephobjectstore",
			},
		}
		updateRTObjects = append(updateRTObjects, cos)
	}

	return updateRTObjects
}

func TestInitStorageClusterResourcesUpdationOnAllPlatforms(t *testing.T) {
	allPlatforms := append(ValidCloudPlatforms,
		PlatformUnknown, CloudPlatformType("NonCloudPlatform"))
	for _, eachPlatform := range allPlatforms {
		cp := &CloudPlatform{platform: eachPlatform}
		initStorageClusterResourceCreateUpdateTestWithPlatform(
			t, cp, createUpdateRuntimeObjects(cp))
	}
}

func assertExpectedResources(t assert.TestingT, reconciler ReconcileStorageCluster, cr *api.StorageCluster, request reconcile.Request) {
	actualSc1 := &storagev1.StorageClass{}
	actualSc2 := &storagev1.StorageClass{}

	request.Name = "ocsinit-cephfs"
	err := reconciler.client.Get(nil, request.NamespacedName, actualSc1)
	assert.NoError(t, err)

	request.Name = "ocsinit-ceph-rbd"
	err = reconciler.client.Get(nil, request.NamespacedName, actualSc2)
	assert.NoError(t, err)

	expected, err := reconciler.newStorageClasses(cr)
	assert.NoError(t, err)

	// The created StorageClasses should not have any ownerReferences set. Any
	// OwnerReference set will be a cross-namespace OwnerReference, which could
	// lead to other child resources getting GCd.
	// Ref: https://bugzilla.redhat.com/show_bug.cgi?id=1755623
	// Ref: https://bugzilla.redhat.com/show_bug.cgi?id=1691546
	assert.Equal(t, len(expected[0].OwnerReferences), 0)
	assert.Equal(t, len(expected[1].OwnerReferences), 0)

	assert.Equal(t, expected[0].ObjectMeta.Name, actualSc1.ObjectMeta.Name)
	assert.Equal(t, expected[0].Provisioner, actualSc1.Provisioner)
	assert.Equal(t, expected[0].ReclaimPolicy, actualSc1.ReclaimPolicy)
	assert.Equal(t, expected[0].Parameters, actualSc1.Parameters)

	assert.Equal(t, expected[1].ObjectMeta.Name, actualSc2.ObjectMeta.Name)
	assert.Equal(t, expected[1].Provisioner, actualSc2.Provisioner)
	assert.Equal(t, expected[1].ReclaimPolicy, actualSc2.ReclaimPolicy)
	assert.Equal(t, expected[1].Parameters, actualSc2.Parameters)

	actualFs := &cephv1.CephFilesystem{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ocsinit-cephfilesystem",
		},
	}
	request.Name = "ocsinit-cephfilesystem"
	err = reconciler.client.Get(nil, request.NamespacedName, actualFs)
	assert.NoError(t, err)

	expectedAf, err := reconciler.newCephFilesystemInstances(cr)
	assert.NoError(t, err)

	assert.Equal(t, len(expectedAf[0].OwnerReferences), 1)

	assert.Equal(t, expectedAf[0].ObjectMeta.Name, actualFs.ObjectMeta.Name)
	assert.Equal(t, expectedAf[0].Spec, actualFs.Spec)

	//
	expectedCosu, err := reconciler.newCephObjectStoreUserInstances(cr)
	assert.NoError(t, err)

	actualCosu := &cephv1.CephObjectStoreUser{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ocsinit-cephobjectstoreuser",
		},
	}
	request.Name = "ocsinit-cephobjectstoreuser"
	err = reconciler.client.Get(nil, request.NamespacedName, actualCosu)
	if reconciler.platform.platform == PlatformAWS {
		assert.Error(t, err)
	} else {
		assert.NoError(t, err)
		assert.Equal(t, expectedCosu[0].ObjectMeta.Name, actualCosu.ObjectMeta.Name)
		assert.Equal(t, expectedCosu[0].Spec, actualCosu.Spec)
	}

	assert.Equal(t, len(expectedCosu[0].OwnerReferences), 1)

	//
	actualCbp := &cephv1.CephBlockPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ocsinit-cephblockpool",
		},
	}
	request.Name = "ocsinit-cephblockpool"
	err = reconciler.client.Get(nil, request.NamespacedName, actualCbp)
	assert.NoError(t, err)

	expectedCbp, err := reconciler.newCephBlockPoolInstances(cr)
	assert.NoError(t, err)

	assert.Equal(t, len(expectedCbp[0].OwnerReferences), 1)

	assert.Equal(t, expectedCbp[0].ObjectMeta.Name, actualCbp.ObjectMeta.Name)
	assert.Equal(t, expectedCbp[0].Spec, actualCbp.Spec)

	//
	expectedCos, err := reconciler.newCephObjectStoreInstances(cr)
	assert.NoError(t, err)

	actualCos := &cephv1.CephObjectStore{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ocsinit-cephobjectstore",
		},
	}
	request.Name = "ocsinit-cephobjectstore"
	err = reconciler.client.Get(nil, request.NamespacedName, actualCos)
	if isValidCloudPlatform(reconciler.platform.platform) {
		assert.Error(t, err)
	} else {
		assert.NoError(t, err)
		assert.Equal(t, expectedCos[0].ObjectMeta.Name, actualCos.ObjectMeta.Name)
		assert.Equal(t, expectedCos[0].Spec, actualCos.Spec)
	}

	assert.Equal(t, len(expectedCos[0].OwnerReferences), 1)
}

func createFakeInitializationStorageClusterReconciler(t *testing.T, obj ...runtime.Object) ReconcileStorageCluster {
	return createFakeInitializationStorageClusterReconcilerWithPlatform(
		t, &CloudPlatform{platform: PlatformUnknown}, obj...)
}

func createFakeInitializationStorageClusterReconcilerWithPlatform(t *testing.T,
	platform *CloudPlatform,
	obj ...runtime.Object) ReconcileStorageCluster {
	scheme := createFakeInitializationScheme(t, obj...)
	obj = append(obj, mockNodeList)
	client := fake.NewFakeClientWithScheme(scheme, obj...)
	if platform == nil {
		platform = &CloudPlatform{platform: PlatformUnknown}
	}

	return ReconcileStorageCluster{
		client:    client,
		scheme:    scheme,
		reqLogger: logf.Log.WithName("controller_storagecluster_test"),
		platform:  platform,
	}
}

func createFakeInitializationScheme(t *testing.T, obj ...runtime.Object) *runtime.Scheme {
	registerObjs := obj
	registerObjs = append(registerObjs)
	api.SchemeBuilder.Register(registerObjs...)
	scheme, err := api.SchemeBuilder.Build()
	if err != nil {
		assert.Fail(t, "unable to build scheme")
	}
	err = corev1.AddToScheme(scheme)
	if err != nil {
		assert.Fail(t, "failed to add corev1 scheme")
	}
	err = cephv1.AddToScheme(scheme)
	if err != nil {
		assert.Fail(t, "failed to add cephv1 scheme")
	}
	err = storagev1.AddToScheme(scheme)
	if err != nil {
		assert.Fail(t, "failed to add storagev1 scheme")
	}
	err = openshiftv1.AddToScheme(scheme)
	if err != nil {
		assert.Fail(t, "failed to add openshiftv1 scheme")
	}
	return scheme
}
