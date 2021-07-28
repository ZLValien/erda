package db

import (
	"errors"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/jinzhu/gorm"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/erda-project/erda-proto-go/core/dicehub/extension/pb"
)

// ExtensionConfig .
type ExtensionConfigDB struct {
	*gorm.DB
}

func (ext *ExtensionVersion) ToApiData(typ string, yamlFormat bool) *pb.ExtensionVersion {
	if yamlFormat {
		return &pb.ExtensionVersion{
			Name:      ext.Name,
			Type:      typ,
			Version:   ext.Version,
			Dice:      structpb.NewStringValue(ext.Dice),
			Spec:      structpb.NewStringValue(ext.Spec),
			Swagger:   structpb.NewStringValue(ext.Swagger),
			Readme:    ext.Readme,
			CreatedAt: timestamppb.New(ext.CreatedAt),
			UpdatedAt: timestamppb.New(ext.UpdatedAt),
			IsDefault: ext.IsDefault,
			Public:    ext.Public,
		}

	} else {
		diceData, _ := yaml.YAMLToJSON([]byte(ext.Dice))
		specData, _ := yaml.YAMLToJSON([]byte(ext.Spec))
		swaggerData, _ := yaml.YAMLToJSON([]byte(ext.Swagger))
		dice := &structpb.Value{}
		dice.UnmarshalJSON(diceData)
		spec := &structpb.Value{}
		spec.UnmarshalJSON(specData)
		swag := &structpb.Value{}
		swag.UnmarshalJSON(swaggerData)
		return &pb.ExtensionVersion{
			Name:      ext.Name,
			Type:      typ,
			Version:   ext.Version,
			Dice:      dice,
			Spec:      spec,
			Swagger:   swag,
			Readme:    ext.Readme,
			CreatedAt: timestamppb.New(ext.CreatedAt),
			UpdatedAt: timestamppb.New(ext.UpdatedAt),
			IsDefault: ext.IsDefault,
			Public:    ext.Public,
		}
	}
}

func (client *ExtensionConfigDB) CreateExtension(extension *Extension) error {
	var cnt int64
	client.Model(&Extension{}).Where("name = ?", extension.Name).Count(&cnt)
	if cnt == 0 {
		err := client.Create(extension).Error
		return err
	} else {
		return errors.New("name already exist")
	}
}

func (client *ExtensionConfigDB) QueryExtensions(all string, typ string, labels string) ([]Extension, error) {
	var result []Extension
	query := client.Model(&Extension{})

	// if all != true,only return data with public = true
	if all != "true" {
		query = query.Where("public = ?", true)
	}

	if typ != "" {
		query = query.Where("type = ?", typ)
	}

	if labels != "" {
		labelPairs := strings.Split(labels, ",")
		for _, pair := range labelPairs {
			if strings.LastIndex(pair, "^") == 0 && len(pair) > 1 {
				query = query.Where("labels not like ?", "%"+pair[1:]+"%")
			} else {
				query = query.Where("labels like ?", "%"+pair+"%")
			}

		}
	}
	err := query.Find(&result).Error
	return result, err
}

func (client *ExtensionConfigDB) GetExtension(name string) (*Extension, error) {
	var result Extension
	err := client.Model(&Extension{}).Where("name = ?", name).Find(&result).Error
	return &result, err
}

func (client *ExtensionConfigDB) DeleteExtension(name string) error {
	return client.Where("name = ?", name).Delete(&Extension{}).Error
}

func (client *ExtensionConfigDB) GetExtensionVersion(name string, version string) (*ExtensionVersion, error) {
	var result ExtensionVersion
	err := client.Model(&ExtensionVersion{}).
		Where("name = ? ", name).
		Where("version = ?", version).
		Find(&result).Error
	return &result, err
}

func (client *ExtensionConfigDB) GetExtensionDefaultVersion(name string) (*ExtensionVersion, error) {
	var result ExtensionVersion
	err := client.Model(&ExtensionVersion{}).
		Where("name = ? ", name).
		Where("is_default = ? ", true).
		Limit(1).
		Find(&result).Error
	//no default,find latest update & public = true
	if err == gorm.ErrRecordNotFound {
		err = client.Model(&ExtensionVersion{}).
			Where("name = ? ", name).
			Where("public = ? ", true).
			Order("version desc").
			Limit(1).
			Find(&result).Error
	}
	return &result, err
}

func (client *ExtensionConfigDB) SetUnDefaultVersion(name string) error {
	return client.Model(&ExtensionVersion{}).
		Where("is_default = ?", true).
		Where("name = ?", name).
		Update("is_default", false).Error
}

func (client *ExtensionConfigDB) CreateExtensionVersion(version *ExtensionVersion) error {
	return client.Create(version).Error
}

func (client *ExtensionConfigDB) DeleteExtensionVersion(name, version string) error {
	return client.Where("name = ? and version =?", name, version).Delete(&ExtensionVersion{}).Error
}

func (client *ExtensionConfigDB) QueryExtensionVersions(name string, all string) ([]ExtensionVersion, error) {
	var result []ExtensionVersion
	query := client.Model(&ExtensionVersion{}).
		Where("name = ?", name)
	// if all != true,only return data with public = true
	if all != "true" {
		query = query.Where("public = ?", true)
	}
	err := query.Find(&result).Error
	return result, err
}

func (client *ExtensionConfigDB) GetExtensionVersionCount(name string) (int64, error) {
	var count int64
	err := client.Model(&ExtensionVersion{}).
		Where("name = ? ", name).
		Count(&count).Error
	return count, err
}
