package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"gorm.io/gorm"
)

type ValidateObjectFunc func(ctx context.Context, obj runtime.Object) error
type ValidateObjectUpdateFunc func(ctx context.Context, newObj, oldObj runtime.Object) error

type UpdatedObjectInfo interface {
	UpdatedObject(ctx context.Context, oldObj runtime.Object) (runtime.Object, error)
}

type simpleUpdatedObjectInfo struct {
	obj runtime.Object
}

func (i *simpleUpdatedObjectInfo) UpdatedObject(ctx context.Context, oldObj runtime.Object) (runtime.Object, error) {
	return i.obj, nil
}

type DBConfig struct {
	Driver          string
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

type DBStorage struct {
	db     *gorm.DB
	scheme *runtime.Scheme
	gvr    schema.GroupVersionResource
}

type ResourceRecord struct {
	ID              uint           `gorm:"primaryKey"`
	Group           string         `gorm:"index:idx_gvr"`
	Version         string         `gorm:"index:idx_gvr"`
	Resource        string         `gorm:"index:idx_gvr"`
	Namespace       string         `gorm:"index:idx_namespace"`
	Name            string         `gorm:"index:idx_name"`
	UID             string         `gorm:"uniqueIndex:idx_uid"`
	RawData         []byte         `gorm:"type:longblob"`
	ResourceVersion string
	Labels          string
	Annotations     string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       gorm.DeletedAt `gorm:"index"`
}

func (ResourceRecord) TableName() string {
	return "container_resources"
}

func NewDBStorage(db *gorm.DB, scheme *runtime.Scheme, gvr schema.GroupVersionResource) *DBStorage {
	return &DBStorage{
		db:     db,
		scheme: scheme,
		gvr:    gvr,
	}
}

func (s *DBStorage) Init() error {
	return s.db.AutoMigrate(&ResourceRecord{})
}

func (s *DBStorage) New() runtime.Object {
	return &metav1.PartialObjectMetadata{}
}

func getObjectName(obj runtime.Object) string {
	if meta, ok := obj.(metav1.Object); ok {
		return meta.GetName()
	}
	return ""
}

func getObjectNamespace(obj runtime.Object) string {
	if meta, ok := obj.(metav1.Object); ok {
		return meta.GetNamespace()
	}
	return ""
}

func getObjectUID(obj runtime.Object) string {
	if meta, ok := obj.(metav1.Object); ok {
		return string(meta.GetUID())
	}
	return ""
}

func getObjectResourceVersion(obj runtime.Object) string {
	if meta, ok := obj.(metav1.Object); ok {
		return meta.GetResourceVersion()
	}
	return ""
}

func getObjectLabels(obj runtime.Object) map[string]string {
	if meta, ok := obj.(metav1.Object); ok {
		return meta.GetLabels()
	}
	return nil
}

func getObjectAnnotations(obj runtime.Object) map[string]string {
	if meta, ok := obj.(metav1.Object); ok {
		return meta.GetAnnotations()
	}
	return nil
}

func (s *DBStorage) Create(ctx context.Context, obj runtime.Object, createValidation ValidateObjectFunc, options *metav1.CreateOptions) (runtime.Object, error) {
	if createValidation != nil {
		if err := createValidation(ctx, obj); err != nil {
			return nil, err
		}
	}

	rawData, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal object: %v", err)
	}

	labels, _ := json.Marshal(getObjectLabels(obj))
	annotations, _ := json.Marshal(getObjectAnnotations(obj))

	record := &ResourceRecord{
		Group:           s.gvr.Group,
		Version:         s.gvr.Version,
		Resource:        s.gvr.Resource,
		Namespace:       getObjectNamespace(obj),
		Name:            getObjectName(obj),
		UID:             getObjectUID(obj),
		RawData:         rawData,
		ResourceVersion: getObjectResourceVersion(obj),
		Labels:          string(labels),
		Annotations:     string(annotations),
	}

	if err := s.db.WithContext(ctx).Create(record).Error; err != nil {
		return nil, fmt.Errorf("failed to create record: %v", err)
	}

	return obj, nil
}

func (s *DBStorage) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	var record ResourceRecord
	query := s.db.WithContext(ctx).Where("group = ? AND version = ? AND resource = ? AND name = ?",
		s.gvr.Group, s.gvr.Version, s.gvr.Resource, name)

	if options != nil && options.ResourceVersion != "" {
		query = query.Where("resource_version = ?", options.ResourceVersion)
	}

	if err := query.First(&record).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("resource %s/%s not found", s.gvr.Resource, name)
		}
		return nil, err
	}

	obj := &metav1.PartialObjectMetadata{}
	if err := json.Unmarshal(record.RawData, obj); err != nil {
		return nil, fmt.Errorf("failed to unmarshal object: %v", err)
	}

	return obj, nil
}

func (s *DBStorage) List(ctx context.Context, options *metav1.ListOptions) (runtime.Object, error) {
	var records []ResourceRecord
	query := s.db.WithContext(ctx).Where("group = ? AND version = ? AND resource = ?",
		s.gvr.Group, s.gvr.Version, s.gvr.Resource)

	if options != nil {
		if options.Limit > 0 {
			query = query.Limit(int(options.Limit))
		}
	}

	if err := query.Find(&records).Error; err != nil {
		return nil, err
	}

	items := make([]metav1.PartialObjectMetadata, 0, len(records))
	for _, record := range records {
		obj := metav1.PartialObjectMetadata{}
		if err := json.Unmarshal(record.RawData, &obj); err != nil {
			continue
		}
		items = append(items, obj)
	}

	return &metav1.PartialObjectMetadataList{
		TypeMeta: metav1.TypeMeta{
			Kind:       s.gvr.Resource + "List",
			APIVersion: s.gvr.GroupVersion().String(),
		},
		Items: items,
	}, nil
}

func (s *DBStorage) Update(ctx context.Context, name string, objInfo UpdatedObjectInfo, createValidation ValidateObjectFunc, updateValidation ValidateObjectUpdateFunc, forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	var record ResourceRecord
	if err := s.db.WithContext(ctx).Where("group = ? AND version = ? AND resource = ? AND name = ?",
		s.gvr.Group, s.gvr.Version, s.gvr.Resource, name).First(&record).Error; err != nil {
		if err == gorm.ErrRecordNotFound && forceAllowCreate {
			obj, err := objInfo.UpdatedObject(ctx, nil)
			if err != nil {
				return nil, false, err
			}
			created, err := s.Create(ctx, obj, createValidation, &metav1.CreateOptions{})
			return created, false, err
		}
		return nil, false, err
	}

	oldObj := &metav1.PartialObjectMetadata{}
	if err := json.Unmarshal(record.RawData, oldObj); err != nil {
		return nil, false, err
	}

	newObj, err := objInfo.UpdatedObject(ctx, oldObj)
	if err != nil {
		return nil, false, err
	}

	if updateValidation != nil {
		if err := updateValidation(ctx, newObj, oldObj); err != nil {
			return nil, false, err
		}
	}

	rawData, err := json.Marshal(newObj)
	if err != nil {
		return nil, false, err
	}

	labels, _ := json.Marshal(getObjectLabels(newObj))
	annotations, _ := json.Marshal(getObjectAnnotations(newObj))

	record.RawData = rawData
	record.ResourceVersion = getObjectResourceVersion(newObj)
	record.Labels = string(labels)
	record.Annotations = string(annotations)

	if err := s.db.WithContext(ctx).Save(&record).Error; err != nil {
		return nil, false, err
	}

	return newObj, false, nil
}

func (s *DBStorage) Delete(ctx context.Context, name string, deleteValidation ValidateObjectFunc, options *metav1.DeleteOptions) (runtime.Object, bool, error) {
	var record ResourceRecord
	if err := s.db.WithContext(ctx).Where("group = ? AND version = ? AND resource = ? AND name = ?",
		s.gvr.Group, s.gvr.Version, s.gvr.Resource, name).First(&record).Error; err != nil {
		return nil, false, err
	}

	obj := &metav1.PartialObjectMetadata{}
	if err := json.Unmarshal(record.RawData, obj); err != nil {
		return nil, false, err
	}

	if deleteValidation != nil {
		if err := deleteValidation(ctx, obj); err != nil {
			return nil, false, err
		}
	}

	if err := s.db.WithContext(ctx).Delete(&record).Error; err != nil {
		return nil, false, err
	}

	return obj, true, nil
}

func (s *DBStorage) DeleteCollection(ctx context.Context, deleteValidation ValidateObjectFunc, options *metav1.DeleteOptions, listOptions *metav1.ListOptions) (runtime.Object, error) {
	list, err := s.List(ctx, listOptions)
	if err != nil {
		return nil, err
	}

	listObj := list.(*metav1.PartialObjectMetadataList)
	for _, item := range listObj.Items {
		_, _, err := s.Delete(ctx, item.Name, deleteValidation, options)
		if err != nil {
			continue
		}
	}

	return list, nil
}

func (s *DBStorage) Watch(ctx context.Context, options *metav1.ListOptions) (watch.Interface, error) {
	return nil, fmt.Errorf("watch not supported for database storage")
}
